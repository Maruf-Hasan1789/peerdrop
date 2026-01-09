package discovery

import "os"

func hostname() string {
	name, err := os.Hostname()

	if err != nil {
		return "unknown"
	}

	return name
}
