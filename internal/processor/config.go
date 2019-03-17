package processor

import (
	"io/ioutil"
	"net/url"
	"nri-flex/internal/load"
	"nri-flex/internal/logger"
	"os"
	"regexp"
	"strings"
	"sync"

	yaml "gopkg.in/yaml.v2"
)

// LoadConfigFiles loads config files
func LoadConfigFiles(ymls *[]load.Config, files []os.FileInfo, path string) {
	for _, f := range files {
		b, err := ioutil.ReadFile(path + f.Name())
		if err != nil {
			logger.Flex("debug", err, "unable to readfile", false)
			continue
		}
		ymlStr := string(b)
		SubEnvVariables(&ymlStr)
		yml, err := ReadYML(ymlStr)
		yml.FileName = f.Name()
		if err != nil {
			logger.Flex("debug", err, "unable to read yml", false)
			continue
		}
		if yml.Name == "" {
			logger.Flex("debug", err, "please set a name on your config file", false)
			// fmt.Println("Please set a name on your config file", f.Name())
			continue
		}
		*ymls = append(*ymls, yml)
	}
}

// ReadYML Unmarshals yml files
func ReadYML(yml string) (load.Config, error) {
	c := load.Config{}
	err := yaml.Unmarshal([]byte(yml), &c)
	if err != nil {
		return load.Config{}, err
	}
	return c, nil
}

// RunConfig Action each config file
func RunConfig(yml load.Config) {
	samplesToMerge := map[string][]interface{}{}
	for i := range yml.APIs {
		dataSets := fetchData(i, &yml)
		runDataHandler(dataSets, &samplesToMerge, i, &yml)
	}
	ProcessSamplesToMerge(&samplesToMerge, &yml)
}

// runLookupProcessor
func runLookupProcessor(str string, cfg *load.Config, i int) {
	lookupReplaces := regexp.MustCompile(`\${lookup:.*?}`).FindAllString(str, -1)

	newConfig := load.Config{
		Name:   cfg.Name,
		Global: cfg.Global,
	}

	for _, lookupReplace := range lookupReplaces {
		// eg. lookupReplace == ${lookup:channels}
		lookupKey := strings.TrimSuffix(strings.Split(lookupReplace, "${lookup:")[1], "}") // eg. "channels"
		if cfg.LookupStore[lookupKey] != nil {
			for _, storedKey := range cfg.LookupStore[lookupKey] { // eg. list of channels
				// add into newConfig>API, and execute processConfig again
				newURL := strings.Replace(str, lookupReplace, url.QueryEscape(storedKey), -1)
				newAPI := load.API{
					Name:      cfg.APIs[i].EventType,
					URL:       newURL,
					EventType: cfg.APIs[i].EventType,
				}
				newConfig.APIs = append(newConfig.APIs, newAPI)
			}
		}
	}

	// re issue process config with newly built config
	RunConfig(newConfig)
}

// YmlFiles Processes yml files
func YmlFiles(ymls *[]load.Config) {
	var wg sync.WaitGroup
	wg.Add(len(*ymls))
	for _, yml := range *ymls {
		go func(yml load.Config) {
			defer wg.Done()
			RunConfig(yml)
			load.ConfigsProcessed++
		}(yml)
	}
	wg.Wait()
}