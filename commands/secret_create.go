// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/openfaas/faas-cli/proxy"
	"github.com/openfaas/faas-cli/schema"
	"github.com/spf13/cobra"
)

var (
	literalSecret string
	secretFile    string
)

// secretCreateCmd represents the secretCreate command
var secretCreateCmd = &cobra.Command{
	Use: `create SECRET_NAME 
			[--from-literal=SECRET_VALUE]
			[--from-file=/path/to/secret/file]
			[STDIN]
			[--tls-no-verify]`,
	Short: "Create a new secret",
	Long:  `The create command creates a new secret from file, literal or STDIN`,
	Example: `faas-cli secret create secret-name --from-literal=secret-value
faas-cli secret create secret-name --from-literal=secret-value --gateway=http://127.0.0.1:8080
faas-cli secret create secret-name --from-file=/path/to/secret/file --gateway=http://127.0.0.1:8080
cat /path/to/secret/file | faas-cli secret create secret-name`,
	RunE:    runSecretCreate,
	PreRunE: preRunSecretCreate,
}

func init() {
	secretCreateCmd.Flags().StringVar(&literalSecret, "from-literal", "", "Value of the secret")
	secretCreateCmd.Flags().StringVar(&secretFile, "from-file", "", "Path to the secret file")
	secretCreateCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	secretCreateCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	secretCmd.AddCommand(secretCreateCmd)
}

func preRunSecretCreate(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("secret name required")
	}

	if len(args) > 1 {
		return fmt.Errorf("too many values for secret name")
	}

	if len(secretFile) > 0 && len(literalSecret) > 0 {
		return fmt.Errorf("please provide secret using only one option from --from-literal, --from-file and STDIN")
	}

	err := validateSecretName(args[0])

	return err
}

func runSecretCreate(cmd *cobra.Command, args []string) error {
	secret := schema.Secret{
		Name: args[0],
	}

	switch {
	case len(literalSecret) > 0:
		secret.Value = literalSecret

	case len(secretFile) > 0:
		var err error
		secret.Value, err = readSecretFromFile(secretFile)
		if err != nil {
			return err
		}

	default:
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			fmt.Fprintf(os.Stderr, "Reading from STDIN - hit (Control + D) to stop.\n")
		}

		secretStdin, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		secret.Value = string(secretStdin)
	}

	secret.Value = strings.TrimSpace(secret.Value)

	if len(secret.Value) == 0 {
		return fmt.Errorf("must provide a non empty secret via --from-literal, --from-file or STDIN")
	}

	gatewayAddress := getGatewayURL(gateway, defaultGateway, "", os.Getenv(openFaaSURLEnvironment))

	if msg := checkTLSInsecure(gatewayAddress, tlsInsecure); len(msg) > 0 {
		fmt.Println(msg)
	}

	fmt.Println("Creating secret: " + secret.Name)
	_, output := proxy.CreateSecret(gatewayAddress, secret, tlsInsecure)
	fmt.Printf(output)

	return nil
}

func readSecretFromFile(secretFile string) (string, error) {
	fileData, err := ioutil.ReadFile(secretFile)
	return string(fileData), err
}

//kubectl create secret generic my_secret --from-literal=my_secret=my_secret
//The Secret "my_secret" is invalid: metadata.name: Invalid value: "my_secret": a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')

// Kubernetes DNS-1123 Subdomain Regex
// https://github.com/kubernetes/kubernetes/blob/6902f3112d98eb6bd0894886ff9cd3fbd03a7f79/staging/src/k8s.io/apimachinery/pkg/util/validation/validation.go#L131
const (
	dns1123LabelFmt     string = "[a-z0-9]([-a-z0-9]*[a-z0-9])?"
	dns1123SubdomainFmt string = dns1123LabelFmt + "(\\." + dns1123LabelFmt + ")*"
)

func validateSecretName(secretName string) error {
	var dns1123SubdomainRegexp = regexp.MustCompile("^" + dns1123SubdomainFmt + "$")

	if !dns1123SubdomainRegexp.MatchString(secretName) {
		return fmt.Errorf(`ERROR: invalid secret name: %s, secret name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (regex used for validation is %s)`, secretName, dns1123SubdomainRegexp)
	}

	return nil
}