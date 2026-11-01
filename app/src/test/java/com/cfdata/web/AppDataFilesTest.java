package com.cfdata.web;

import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;

public final class AppDataFilesTest {
    public static void main(String[] args) throws Exception {
        File root = Files.createTempDirectory("cfdata-android-storage-test-").toFile();
        try {
            File legacy = new File(root, "legacy");
            File target = new File(root, "Download/cfdata-web");
            AppDataFiles.prepareDirectory(legacy);
            AppDataFiles.prepareDirectory(target);
            Files.write(new File(legacy, "huawei-dns.private").toPath(), new byte[]{1, 2, 3});
            Files.write(new File(legacy, "latest-results.json").toPath(), "old results".getBytes(StandardCharsets.UTF_8));
            Files.write(new File(target, "latest-results.json").toPath(), "new results".getBytes(StandardCharsets.UTF_8));
            Files.write(new File(legacy, "previous-results.csv").toPath(), "old export".getBytes(StandardCharsets.UTF_8));
            Files.write(new File(legacy, "libcfdata.so").toPath(), new byte[]{9});
            AppDataFiles.migrateLegacyFiles(legacy, target);
            check(Arrays.equals(Files.readAllBytes(new File(target, "huawei-dns.private").toPath()), new byte[]{1, 2, 3}), "credentials were not migrated");
            check(new String(Files.readAllBytes(new File(target, "latest-results.json").toPath()), StandardCharsets.UTF_8).equals("new results"), "new results were overwritten");
            check(new File(legacy, "huawei-dns.private").isFile(), "migration deleted its source");
            check(new File(target, "previous-results.csv").isFile(), "old export was lost");
            check(!new File(target, "libcfdata.so").exists(), "executable moved into shared storage");
            AppDataFiles.migrateLegacyFiles(legacy, target);

            File first = AppDataFiles.writeExport(target, "results.csv", "IP\n192.0.2.1");
            File second = AppDataFiles.writeExport(target, "results.csv", "IP\n192.0.2.2");
            check(!first.equals(second), "export name collision overwrote results");
            byte[] bytes = Files.readAllBytes(first.toPath());
            check(bytes[0] == (byte) 0xEF && bytes[1] == (byte) 0xBB && bytes[2] == (byte) 0xBF, "UTF-8 BOM lost");
            check(new String(bytes, 3, bytes.length - 3, StandardCharsets.UTF_8).equals("IP\n192.0.2.1"), "first export changed");
            java.util.concurrent.ExecutorService writers = java.util.concurrent.Executors.newFixedThreadPool(4);
            try {
                java.util.concurrent.CountDownLatch ready = new java.util.concurrent.CountDownLatch(1);
                java.util.List<java.util.concurrent.Future<File>> outputs = new java.util.ArrayList<>();
                for (int index = 0; index < 24; index++) {
                    final int number = index;
                    outputs.add(writers.submit(() -> {
                        ready.await();
                        return AppDataFiles.writeExport(target, "concurrent.csv", "result " + number);
                    }));
                }
                ready.countDown();
                java.util.Set<File> unique = new java.util.HashSet<>();
                for (int index = 0; index < outputs.size(); index++) {
                    File exported = outputs.get(index).get();
                    check(unique.add(exported), "concurrent export reused an existing file");
                    byte[] result = Files.readAllBytes(exported.toPath());
                    check(new String(result, 3, result.length - 3, StandardCharsets.UTF_8).equals("result " + index), "concurrent export was overwritten");
                }
            } finally {
                writers.shutdownNow();
            }
            for (String name : new String[]{"../outside.txt", "..", ".", "", null, "a/b\\c.txt"}) {
                File exported = AppDataFiles.writeExport(target, name, "safe");
                check(exported.getCanonicalFile().getParentFile().equals(target.getCanonicalFile()), "export escaped the data directory");
            }
            File blocker = new File(root, "not-a-directory");
            Files.write(blocker.toPath(), new byte[]{7});
            try {
                AppDataFiles.writeExport(blocker, "results.csv", "must not overwrite");
                throw new AssertionError("export into a file unexpectedly succeeded");
            } catch (java.io.IOException expected) {
                check(Arrays.equals(Files.readAllBytes(blocker.toPath()), new byte[]{7}), "failed export damaged an existing file");
            }
            check(Arrays.stream(target.list()).noneMatch(name -> name.startsWith(".cfdata-write-check-")), "directory write probe leaked");
            System.out.println("PASS: Android migration, sequential/concurrent collision-safe exports, BOM, traversal rejection and failed writes");
        } finally {
            try (java.util.stream.Stream<Path> paths = Files.walk(root.toPath())) {
                for (Path path : paths.sorted(java.util.Comparator.reverseOrder()).toArray(Path[]::new)) {
                    Files.delete(path);
                }
            }
        }
    }

    private static void check(boolean condition, String message) {
        if (!condition) {
            throw new AssertionError(message);
        }
    }
}
