package parser

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var definitionsMap map[string]interface{}

// PopulateKubeAPIMap Converts an API Definition into a map of KubeAPIs["group/version/kind"]
func (kubeAPIs KubernetesAPIs) PopulateKubeAPIMap(swaggerfile string) (err error) {
	// Open our jsonFile
	log.Debugf("Opening the swagger file for reading: %s", swaggerfile)
	jsonFile, err := os.Open(swaggerfile)
	// if we os.Open returns an error then handle it
	if err != nil {
		return err
	}
	// read our opened jsonFile as a byte array.
	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return err
	}

	err = jsonFile.Close()
	if err != nil {
		return err
	}

	err = json.Unmarshal(byteValue, &definitionsMap)
	if err != nil {
		return fmt.Errorf("error parsing the JSON, file might be invalid: %v", err)
	}

	definitions := definitionsMap["definitions"].(map[string]interface{}) // nolint: errcheck

	log.Debugf("Iteracting through %d definitions", len(definitions))
	for k, value := range definitions {
		val := value.(map[string]interface{}) // nolint: errcheck
		log.Debugf("Getting API values from %s", k)

		if kubeapivalue, valid := getKubeAPIValues(val); valid {
			log.Debugf("Valid API object found for %s", k)

			var name string
			if kubeapivalue.Group != "" {
				name = fmt.Sprintf("%s/%s/%s", kubeapivalue.Group, kubeapivalue.Version, kubeapivalue.Kind)
			} else {
				name = fmt.Sprintf("%s/%s", kubeapivalue.Version, kubeapivalue.Kind)
			}

			log.Debugf("Adding %s to map. Deprecated: %t", name, kubeapivalue.Deprecated)
			kubeAPIs[name] = kubeapivalue
		}
	}

	return nil
}

func getGroupVersionKind(value map[string]interface{}) (group, version, kind string) {
	for k, v := range value {
		switch k {
		case "group":
			group = v.(string) //nolint: errcheck
		case "version":
			version = v.(string) //nolint: errcheck
		case "kind":
			kind = v.(string) //nolint: errcheck
		}
	}

	return group, version, kind
}

func getKubeAPIValues(value map[string]interface{}) (KubeAPI, bool) {
	var (
		valid, deprecated                 bool
		description, group, version, kind string
	)

	gvk, valid, err := unstructured.NestedSlice(value, "x-kubernetes-group-version-kind")

	if !valid || err != nil {
		return KubeAPI{}, false
	}

	gvkMap := gvk[0]
	group, version, kind = getGroupVersionKind(gvkMap.(map[string]interface{}))

	description, found, err := unstructured.NestedString(value, "description")

	if !found || err != nil || description == "" {
		log.Debugf("Marking the resource as invalid because it doesn't contain a description")
		return KubeAPI{}, false
	}

	if strings.Contains(strings.ToLower(description), "deprecated") {
		log.Debugf("API Definition contains the word DEPRECATED in its description")
		deprecated = true
	}

	if valid {
		return KubeAPI{
			Description: description,
			Group:       group,
			Kind:        kind,
			Version:     version,
			Deprecated:  deprecated,
		}, true
	}

	return KubeAPI{}, false
}
