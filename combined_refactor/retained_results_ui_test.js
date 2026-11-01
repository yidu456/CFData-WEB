const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const html = fs.readFileSync(`${__dirname}/index.html`, 'utf8');
function extract(name) {
  const start = html.indexOf(`        function ${name}(`);
  assert.ok(start >= 0, name);
  const end = html.indexOf('\n        function ', start + 1);
  return html.slice(start, end < 0 ? html.indexOf('</script>', start) : end);
}
const state = vm.createContext({
  testData: [], retainedTestIndex: new Map(), retainedTestArray: null, retainedTestLength: 0,
  officialSpeedMinValue: 2.5, getOfficialRowSpeed: row => row.Speed || row.speed,
  normalizeQualifiedFilter: value => value, parseSpeedMB: value => parseFloat(value) || -1,
});
for (const name of ['mergeRetainedTests', 'isOfficialSpeedQualified', 'getSpeedStatus', 'matchesOfficialQualifiedFilter', 'officialResultQualified']) vm.runInContext(extract(name), state);
vm.runInContext(`
  mergeRetainedTests([{IP:'198.51.100.1',Port:443,Speed:'4MB/s',ExactSpeed:{measured:true,mb:4}}]);
  mergeRetainedTests([{IP:'198.51.100.1',Port:443,AvgLatency:1000000}]);
`, state);
assert.equal(state.testData[0].Speed, '4MB/s', 'latency retests preserve previous speed');
assert.equal(state.testData[0].ExactSpeed.mb, 4);
vm.runInContext(`mergeRetainedTests([{IP:'198.51.100.1',Port:8443,Speed:'1MB/s'}])`, state);
assert.equal(state.testData.length, 2, 'same IP different port stays separate');
vm.runInContext(`testData=testData.slice().reverse();mergeRetainedTests([{IP:'198.51.100.1',Port:443,Speed:'failed'}])`, state);
assert.equal(state.testData.find(row => row.Port === 443).Speed, 'failed');
assert.equal(state.testData.find(row => row.Port === 8443).Speed, '1MB/s');
assert.equal(vm.runInContext(`matchesOfficialQualifiedFilter({Speed:'2.50MB/s',ExactSpeed:{measured:true,mb:2.499}},'qualified')`, state), false, 'rounded speed cannot change qualification');
assert.equal(vm.runInContext(`matchesOfficialQualifiedFilter({speed:'2.50MB/s',ExactSpeed:{measured:true,mb:2.501}},'qualified')`, state), true);
assert.equal(vm.runInContext(`officialResultQualified({Speed:'failed',ExactSpeed:{measured:false,mb:0}})`, state), false);
state.testData = [];
state.retainedTestArray = null;
const start = performance.now();
for (let i = 0; i < 100000; i++) state.mergeRetainedTests([{ IP: `sample-${i}`, Port: 443, Speed: '' }]);
assert.equal(state.testData.length, 100000);
assert.ok(performance.now() - start < 5000, 'incremental merge must not rebuild whole pool per row');
console.log('Retained result merge, endpoint separation, exact qualification and 100000-row incremental updates PASS');
