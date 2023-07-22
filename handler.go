package main

import (
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"net/http"
)

func ReadRemoteConfig(w http.ResponseWriter, r *http.Request) {
	response, err := repository.getData(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	isValid := isValidYAMLFile(response)

	if !isValid {
		logrus.WithError(err).Error("error validating yaml")
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
