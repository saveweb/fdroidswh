package main

import (
	"encoding/json"
	"errors"
)

type PackageInfo struct {
	Metadata Metadata `json:"metadata"`
}

type Metadata struct {
	Added       int64  `json:"added"`
	LastUpdated int64  `json:"lastUpdated"`
	SourceCode  string `json:"sourceCode"`
}

func convertToPackageInfo(packageData any) (*PackageInfo, error) {
	packageMap, ok := packageData.(map[string]any)
	if !ok {
		return nil, errors.New("package data is not a map")
	}

	metadataData, ok := packageMap["metadata"].(map[string]any)
	if !ok {
		return nil, errors.New("metadata field missing or invalid")
	}

	added, ok := metadataData["added"].(float64)
	if !ok {
		return nil, errors.New("added field missing or invalid")
	}

	lastUpdated, ok := metadataData["lastUpdated"].(float64)
	if !ok {
		return nil, errors.New("lastUpdated field missing or invalid")
	}

	sourceCode, ok := metadataData["sourceCode"].(string)
	if !ok {
		sourceCode = "" // Use a default value (empty string)
	}

	packageInfo := &PackageInfo{
		Metadata: Metadata{
			Added:       int64(added),
			LastUpdated: int64(lastUpdated),
			SourceCode:  sourceCode,
		},
	}
	return packageInfo, nil
}

func ParseIndex(data []byte) (map[string]PackageInfo, error) {
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, errors.Join(err, errors.New("failed to parse the json data"))
	}

	packages, ok := result["packages"].(map[string]any)
	if !ok {
		return nil, errors.New("packages field missing")
	}

	packageInfos := make(map[string]PackageInfo)

	for packageName, packageData := range packages {
		packageInfo, err := convertToPackageInfo(packageData)
		if err != nil {
			panic(err)
		}
		packageInfos[packageName] = *packageInfo
	}

	return packageInfos, nil
}
