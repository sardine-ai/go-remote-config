package main

import (
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"net/http"
)

var output string

func ReadRemoteConfig(w http.ResponseWriter, r *http.Request) {
	response, err := repository.getData()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = w.Write([]byte(response))
	if err != nil {
		logrus.WithError(err).Error("error writing response")
	}
}

func isValidYAMLFile(response string) bool {
	// Attempt to unmarshal the content into an interface{}.
	// If the unmarshal is successful, it means the file contains valid YAML data.
	var data map[string]interface{}
	err := yaml.Unmarshal([]byte(response), &data)
	if err != nil {
		logrus.WithError(err).Error("error unmarshalling yaml")
		return false
	}
	return true
}
