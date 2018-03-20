package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/command"
	"log"
	"os"
	"reflect"
	"strings"
)

var verboseMode bool
var Version string
var GitCommit string

const VaultPathEnvPrefix = "vault:"

// Logical interface is just what we expect to use from Vault
type Logical interface {
	Read(string) (*api.Secret, error)
}

type VaultPath struct {
	Path string
	Key  string
}

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

func awsEnvVars(logical Logical, path string) (map[string]string, error) {
	environ := []string{
		fmt.Sprintf("AWS_ACCESS_KEY_ID=vault:%s!!!access_key", path),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=vault:%s!!!secret_key", path),
		fmt.Sprintf("AWS_SESSION_TOKEN=vault:%s!!!security_token", path),
	}
	sep := "!!!"
	return substituteVaultPaths(logical, environ, sep)

}

func printEnvCode(env map[string]string) {
	for k, v := range env {
		fmt.Printf("export %s=%s\n", k, v)
	}
}

// Grab all the environment variables from the environment
func findVarsToFillInFromEnv(environ []string) map[string]string {
	out := make(map[string]string)
	for _, envvar := range environ {
		pair := strings.SplitN(envvar, "=", 2)
		if strings.HasPrefix(pair[1], VaultPathEnvPrefix) {
			out[pair[0]] = pair[1]
		}
	}
	return out
}

// split a path into 2 parts, the vault path and possibly the key name (only if
// keySep is defined)
func splitRawPath(rawVar string, keySep string) VaultPath {
	path := rawVar[len(VaultPathEnvPrefix):]
	key := ""
	if keySep != "" {
		split := strings.SplitN(path, keySep, 2)
		if len(split) == 2 {
			path = split[0]
			key = split[1]
		}
	}
	return VaultPath{path, key}
}

// lookup path value from logical, pulling from cache first if possible.
// if lookup happens, populates cache with looked up value.
func lookupPath(logical Logical, path VaultPath, cache map[string]map[string]interface{}) (string, error) {
	if cache == nil {
		// should this be a panic?
		return "", errors.New("cache cannot be nil :-/")
	}
	if _, present := cache[path.Path]; !present {
		secret, err := logical.Read(path.Path)
		if err != nil {
			return "", err
		}
		cache[path.Path] = secret.Data
	}
	getKeys := func(m map[string]interface{}) []string {
		secretKeys := make([]string, 0)
		for k, _ := range m {
			secretKeys = append(secretKeys, k)
		}
		return secretKeys
	}
	secretData := cache[path.Path]
	var value interface{}
	if path.Key != "" {
		if _, ok := secretData[path.Key]; !ok {
			return "", fmt.Errorf("No key '%s' at path '%s'. Keys were: '%s'",
				path.Key, path.Path, strings.Join(getKeys(secretData), ","))
		}
		value = secretData[path.Key]
	} else {
		if len(secretData) != 1 {
			return "", fmt.Errorf("Found multiple keys at path '%s'. Keys were: '%s'",
				path.Path, strings.Join(getKeys(secretData), ","))
		}
		for k, v := range secretData {
			// fill in the key for error message below
			path.Key = k
			value = v
			break
		}
	}
	if value == nil {
		return "", nil
	}
	unwrapped, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("Invalid type for key '%s' at path '%s' (%s)",
			path.Key, path.Path, reflect.TypeOf(value))
	}
	return unwrapped, nil
}

// substitute vaultPaths into envvars from environ by reading from given Logical.
// if multiple env vars request the same path, it will only be read once!
// (allows for pulling from the same aws creds for two different env vars).
// E.g.
// AWS_ACCESS_KEY_ID=aws/creds/myrole:access_key
// AWS_SECRET_ACCESS_KEY=aws/creds/myrole:secret_key
// AWS_SECURITY_TOKEN=aws/creds/myrole:security_token
func substituteVaultPaths(logical Logical, environ []string, keySep string) (map[string]string, error) {
	vaultMap := findVarsToFillInFromEnv(environ)
	cache := make(map[string]map[string]interface{}, len(vaultMap))
	outMap := make(map[string]string, len(vaultMap))
	for envvar, path := range vaultMap {
		parsed := splitRawPath(path, keySep)
		unwrapped, err := lookupPath(logical, parsed, cache)
		if err != nil {
			err = fmt.Errorf("%s (%+v): %s", envvar, parsed, err)
			return nil, err
		}
		outMap[envvar] = unwrapped
	}
	return outMap, nil
}

func main() {
	flag.BoolVar(&verboseMode, "verbose", false, "Enable debug output")
	version := flag.Bool("version", false, "display version and exit")
	keySep := flag.String("keysep", "", "Separator for vault path from vault key")
	// prevent all of vault's CLI params from leaking through
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-verbose] [-keysep SEP] [aws/creds/<path>]\n", os.Args[0])
		fmt.Fprint(os.Stderr, "  -verbose\n\tEnable debug output\n")
		fmt.Fprint(os.Stderr, "  -version\n\tDisplay version and exit\n")
		fmt.Fprint(os.Stderr, "  -keysep\n\tSeparator between key and path for vault\n")
		fmt.Fprint(os.Stderr, " (if no path supplied supplied substitutes entire environment\n")
		fmt.Fprint(os.Stderr, "  Environment Variables:\n")
		fmt.Fprint(os.Stderr, "\tVAULT_ADDR: set vault hostname\n\tVAULT_TOKEN: use a specific token to auth to vault\n")
	}

	flag.Parse()
	if *version {
		fmt.Printf("%s\n  Version: %s\n  Commit: %s\n", os.Args[0], Version, GitCommit)
		os.Exit(0)
	}
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	path := flag.Arg(0)
	client, err := api.NewClient(nil)
	if err != nil {
		log.Fatalf("Failed to generate client: %s", err)
	}
	maybeSetTokenFromHelper(client)

	var env map[string]string

	if path != "" {
		env, err = awsEnvVars(client.Logical(), path)
	} else {
		env, err = substituteVaultPaths(client.Logical(), os.Environ(), *keySep)
	}
	if err != nil {
		// show standard vault error, already nicely formatted
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	printEnvCode(env)
}
