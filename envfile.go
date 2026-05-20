package ecspresso

import (
	"os"

	"github.com/hashicorp/go-envparse"
)

// exportEnvFile exports envfile to environment variables.
func exportEnvFile(file string) error {
	if file == "" {
		return nil
	}
	LogDebug("loading envfile: %s", file)

	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	envs, err := envparse.Parse(f)
	if err != nil {
		return err
	}
	for key, value := range envs {
		os.Setenv(key, value)
	}
	return nil
}
