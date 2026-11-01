package main

import (
	"errors"
	"sort"
)

func resolveDetailDatacenters(s *appSession, selected []string) ([]string, error) {
	if len(selected) == 0 || len(selected) > 256 {
		return nil, errors.New("select between 1 and 256 discovered datacenters")
	}
	available := make(map[string]bool)
	s.scanMutex.Lock()
	for _, row := range s.scanResults {
		available[row.DataCenter] = true
	}
	s.scanMutex.Unlock()
	unique := make(map[string]bool)
	for _, dc := range selected {
		if !validRetainedDC(dc) || !available[dc] {
			return nil, errors.New("datacenter is not present in the current scan")
		}
		unique[dc] = true
	}
	result := make([]string, 0, len(unique))
	for dc := range unique {
		result = append(result, dc)
	}
	sort.Strings(result)
	return result, nil
}
