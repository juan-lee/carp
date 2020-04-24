package azure

import (
	"fmt"

	"github.com/Azure/go-autorest/autorest/azure/auth"
)

// GetSettings returns unstructured azure settings for the given environment
func GetSettings() (map[string]string, error) {
	file, fileErr := auth.GetSettingsFromFile()
	if fileErr != nil {
		env, envErr := auth.GetSettingsFromEnvironment()
		if envErr != nil {
			return nil, fmt.Errorf("failed to get settings from file: %s\n\n failed to get settings from environment: %s", fileErr.Error(), envErr.Error())
		}
		return env.Values, nil
	}
	return file.Values, nil
}
