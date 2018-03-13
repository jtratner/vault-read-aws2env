package main

import (
	"flag"
	"fmt"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/command"
	"log"
	"os"
)

var verboseMode bool

func logDebug(format string, v ...interface{}) {
	if verboseMode {
		log.Printf(format, v...)
	}
}

func maybeSetTokenFromHelper(c *api.Client) {
	if c.Token() != "" {
		return
	}
	logDebug("attempting to pull token manually")
	helper, err := command.DefaultTokenHelper()
	if err != nil {
		logDebug("cannot use token helper. Error: %s", err)
		return
	}
	token, err := helper.Get()
	if err != nil {
		logDebug("token helper could not generate token: %s", err)
		return
	}
	c.SetToken(token)
	logDebug("managed to set token from helper")
}

// converts an api.Secret's data to a mapping of environment variable => value
func secretData2EnvMapping(data map[string]interface{}) map[string]string {
	getStringMapValue := func(key string) (string, bool) {
		wrapped_value, ok := data[key]
		if !ok {
			return "", false
		}
		value, ok := wrapped_value.(string)
		if !ok {
			return "", false
		}
		return value, ok

	}
	out := make(map[string]string)
	if access_key, ok := getStringMapValue("access_key"); ok {
		out["AWS_ACCESS_KEY_ID"] = access_key
	} else {
		log.Fatalf("No access key on request!")
	}
	if secret_key, ok := getStringMapValue("secret_key"); ok {
		out["AWS_SECRET_ACCESS_KEY"] = secret_key
	} else {
		log.Fatalf("No secret key on request!")
	}
	// for some reason backend names differently
	if security_token, ok := getStringMapValue("security_token"); ok {
		out["AWS_SESSION_TOKEN"] = security_token
	}
	return out
}

func printEnvCode(env map[string]string) {
	for k, v := range env {
		fmt.Printf("export %s='%s'\n", k, v)
	}
}

func main() {
	flag.BoolVar(&verboseMode, "verbose", false, "Enable debug output")
	// prevent all of vault's CLI params from leaking through
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-verbose] aws/creds/<path>\n", os.Args[0])
		fmt.Fprint(os.Stderr, "  -verbose\n\tEnable debug output\n")
		fmt.Fprint(os.Stderr, "  Environment Variables:\n")
		fmt.Fprint(os.Stderr, "\tVAULT_ADDR: set vault hostname\n\tVAULT_TOKEN: use a specific token to auth to vault\n")
	}

	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	path := flag.Arg(0)
	client, err := api.NewClient(nil)
	if err != nil {
		log.Fatalf("Failed to generate client: %s", err)
	}
	maybeSetTokenFromHelper(client)

	secret, err := client.Logical().Read(path)
	if err != nil {
		// show standard vault error, already nicely formatted
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	env := secretData2EnvMapping(secret.Data)
	printEnvCode(env)
}
