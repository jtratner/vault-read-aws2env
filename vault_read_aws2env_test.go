package main

import (
	"errors"
	"fmt"
	"github.com/hashicorp/vault/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type DummyLogical struct {
	pathMap map[string]map[string]interface{}
}

const NotFoundErrorMessage = "NOT FOUND"

func (c *DummyLogical) Read(p string) (*api.Secret, error) {
	if value, ok := c.pathMap[p]; ok {
		return &api.Secret{Data: value}, nil
	}
	return nil, errors.New(NotFoundErrorMessage)
}

var _ = Describe("VaultReadAws2env", func() {
	Context("findVarsToFillInFromEnv", func() {
		It("should grab no env vars when they aren't prefixed with vault", func() {
			Expect(findVarsToFillInFromEnv([]string{"MYKEY=value"})).To(Equal(map[string]string{}))
		})
		It("should only find envvars that start with vault", func() {
			Expect(findVarsToFillInFromEnv([]string{"MYKEY=svault:whatever", "SOMEVALUE=vault:excitingpath"})).To(Equal(map[string]string{
				"SOMEVALUE": "vault:excitingpath",
			}))
		})
		It("should not error when no env vars", func() {
			Expect(findVarsToFillInFromEnv([]string{})).To(Equal(map[string]string{}))
		})
		It("should handle multiple = signs", func() {
			Expect(findVarsToFillInFromEnv([]string{"K=vault:a=1=2"})).To(Equal(map[string]string{
				"K": "vault:a=1=2",
			}))
		})
	})
	errorShouldMatch := func(err error, val interface{}, regexp string) {
		Expect(err).ToNot(BeNil(), fmt.Sprintf("error was nil rather than matching %s (value was %v)", regexp, val))
		Expect(err.Error()).To(MatchRegexp(regexp))
	}
	emptyCache := make(map[string]map[string]interface{})
	expectEmptyCache := func() {
		Expect(len(emptyCache)).To(Equal(0), "item ended up in cache")
	}
	Context("lookupPath", func() {
		It("should only read each value once", func() {
			cache := map[string]map[string]interface{}{
				"mypath": map[string]interface{}{
					"somekey": "myvalue",
				}}
			Expect(lookupPath(&DummyLogical{},
				VaultPath{"mypath", ""}, cache)).To(Equal("myvalue"))
			expectEmptyCache()
		})
		It("should error if path cannot be found", func() {
			val, err := lookupPath(&DummyLogical{}, VaultPath{"path", ""}, emptyCache)
			errorShouldMatch(
				err, val,
				NotFoundErrorMessage)
			expectEmptyCache()
		})
		It("should error if key is not found", func() {
			cache := make(map[string]map[string]interface{})
			val, err := lookupPath(&DummyLogical{map[string]map[string]interface{}{
				"mypath": map[string]interface{}{
					"anotherkey": "whatever",
				},
			}}, VaultPath{"mypath", "mykey"},
				cache)
			errorShouldMatch(
				err, val,
				"No key 'mykey' at path 'mypath'. Keys were: 'anotherkey'",
			)
			Expect(len(cache)).To(Equal(1))
		})
		It("should error when you try to read a key not present", func() {
			val, err := lookupPath(&DummyLogical{}, VaultPath{"path", "key"}, emptyCache)
			errorShouldMatch(
				err, val,
				NotFoundErrorMessage)
			expectEmptyCache()
		})
		It("should pull the key if only one key is present in Data", func() {
			Expect(lookupPath(&DummyLogical{map[string]map[string]interface{}{
				"mypath": map[string]interface{}{
					"somekey": "myvalue",
				}}},
				VaultPath{"mypath", ""}, make(map[string]map[string]interface{}))).To(Equal("myvalue"))
			expectEmptyCache()
		})
		It("should error if key is unspecified and there are 2+ keys", func() {
			val, err := lookupPath(&DummyLogical{map[string]map[string]interface{}{
				"mypath": map[string]interface{}{
					"somekey":    "myvalue",
					"anotherkey": "blah",
				}}},
				VaultPath{"mypath", ""}, make(map[string]map[string]interface{}))
			errorShouldMatch(
				err, val,
				"Found multiple keys.*mypath.*anotherkey")
		})
		It("should pull the specified key", func() {
			Expect(lookupPath(&DummyLogical{map[string]map[string]interface{}{
				"mypath": map[string]interface{}{
					"somekey":    "myvalue",
					"anotherkey": "anothervalue",
				}}},
				VaultPath{"mypath", "somekey"}, make(map[string]map[string]interface{}))).To(Equal("myvalue"))
		})
		It("should return \"\" for nil values", func() {
			Expect(lookupPath(&DummyLogical{map[string]map[string]interface{}{
				"mypath": map[string]interface{}{
					"somekey":    nil,
					"anotherkey": "anothervalue",
				}}},
				VaultPath{"mypath", "somekey"}, make(map[string]map[string]interface{}))).To(Equal(""))
		})
	})
	Context("splitRawPath", func() {
		It("should just strip vault without keySep", func() {
			Expect(splitRawPath("vault:secret/blah", "")).To(Equal(VaultPath{"secret/blah", ""}))
		})
		It("should strip vault with keySep if not in path", func() {
			Expect(splitRawPath("vault:secret/blah", ":")).To(Equal(VaultPath{"secret/blah", ""}))
		})
		It("should split with keySep", func() {
			Expect(splitRawPath("vault:aws/creds/myrole:secret_key", ":")).To(Equal(VaultPath{"aws/creds/myrole", "secret_key"}))
		})
		It("should split on first instance", func() {
			Expect(splitRawPath("vault:something/really/big!exciting!stuff", "!")).To(Equal(VaultPath{"something/really/big", "exciting!stuff"}))
		})
	})
	Context("awsEnvVars", func() {
		path := "aws/creds/myrole"
		It("should convert nil to empty string", func() {
			Expect(awsEnvVars(&DummyLogical{map[string]map[string]interface{}{
				path: map[string]interface{}{
					"access_key":     "abc",
					"secret_key":     "mysecret",
					"security_token": nil,
				}}}, path)).To(Equal(map[string]string{
				"AWS_ACCESS_KEY_ID":     "abc",
				"AWS_SECRET_ACCESS_KEY": "mysecret",
				"AWS_SESSION_TOKEN":     "",
			}))
		})
		It("should convert all args", func() {
			Expect(awsEnvVars(&DummyLogical{map[string]map[string]interface{}{
				path: map[string]interface{}{
					"access_key":     "abc",
					"secret_key":     "mysecret",
					"security_token": "apple",
				}}}, path)).To(Equal(map[string]string{
				"AWS_ACCESS_KEY_ID":     "abc",
				"AWS_SECRET_ACCESS_KEY": "mysecret",
				"AWS_SESSION_TOKEN":     "apple",
			}))
		})
		It("should error usefully when path not found", func() {
			val, err := awsEnvVars(&DummyLogical{}, path)
			errorShouldMatch(
				err, val, ".*aws/creds/myrole.*NOT FOUND")
		})
	})
})
