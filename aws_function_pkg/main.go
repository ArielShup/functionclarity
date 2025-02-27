// Copyright © 2022 Cisco Systems, Inc. and its affiliates.
// All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/openclarity/function-clarity/pkg/clients"
	i "github.com/openclarity/function-clarity/pkg/init"
	"github.com/openclarity/function-clarity/pkg/integrity"
	opts "github.com/openclarity/function-clarity/pkg/options"
	"github.com/openclarity/function-clarity/pkg/verify"
	co "github.com/sigstore/cosign/cmd/cosign/cli/options"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"os"
	"strings"
)

type ResponseElement struct {
	FunctionName string `json:"functionName"`
	FunctionArn  string `json:"functionArn"`
}

type RecordMessage struct {
	AwsRegion        string          `json:"awsRegion"`
	EventSource      string          `json:"eventSource"`
	EventName        string          `json:"eventName"`
	ResponseElements ResponseElement `json:"responseElements"`
}

type Record struct {
	Message string `json:"message"`
	Id      string `json:"id"`
}

type FilterRecord struct {
	LogEvents   []Record `json:"logEvents"`
	MessageType string   `json:"messageType"`
}

var config *i.AWSInput = nil

func HandleRequest(context context.Context, cloudWatchEvent events.CloudwatchLogsEvent) error {
	filterRecord, err := extractDataFromEvent(cloudWatchEvent)
	if err != nil {
		log.Printf("Failed to extract data from event: %v", err)
		return fmt.Errorf("failed to extract data from event: %w", err)
	}
	recordMessage := RecordMessage{}
	logEvents := filterRecord.LogEvents
	if config == nil {
		err := initConfig()
		if err != nil {
			return err
		}
	}
	for logEvent := range logEvents {
		err = json.Unmarshal([]byte(logEvents[logEvent].Message), &recordMessage)
		if err != nil {
			log.Printf("failed to extract message from event, skipping message. %s", logEvents[logEvent].Message)
			continue
		}
		if shouldHandleEvent(recordMessage) {
			log.Printf("handling function name: %s, event name: %s, event source: %s, region: %s\n", recordMessage.ResponseElements.FunctionName, recordMessage.EventName, recordMessage.EventSource, recordMessage.AwsRegion)
			handleFunctionEvent(recordMessage, config.IncludedFuncTagKeys, config.IncludedFuncRegions, context)
		}
	}

	return nil
}

func shouldHandleEvent(recordMessage RecordMessage) bool {
	return (strings.Contains(recordMessage.EventName, "CreateFunction") || strings.Contains(recordMessage.EventName, "UpdateFunctionCode")) &&
		"FunctionClarityLambdaVerifier" != recordMessage.ResponseElements.FunctionName && "" != recordMessage.ResponseElements.FunctionName
}

func handleFunctionEvent(recordMessage RecordMessage, tagKeysFilter []string, regionsFilter []string, ctx context.Context) {
	awsClientForDocker := clients.NewAwsClient("", "", config.Bucket, recordMessage.AwsRegion, recordMessage.AwsRegion)
	err := integrity.InitDocker(awsClientForDocker)
	if err != nil {
		log.Printf("Failed to init docker. %v", err)
		return
	}
	o := getVerifierOptions(config.IsKeyless, config.PublicKey)
	log.Printf("about to execute verification with post action: %s.", config.Action)
	awsClient := clients.NewAwsClient("", "", config.Bucket, config.Region, recordMessage.AwsRegion)
	err = verify.Verify(awsClient, recordMessage.ResponseElements.FunctionName, o, ctx, config.Action, config.SnsTopicArn, config.Region, tagKeysFilter, regionsFilter)

	if err != nil {
		log.Printf("Failed to handle lambda result: %s, %v", recordMessage.ResponseElements.FunctionArn, err)
	}
}

func initConfig() error {
	envConfig := os.Getenv("CONFIGURATION")
	log.Printf("config: %s", envConfig)
	decodedConfig, err := base64.StdEncoding.DecodeString(envConfig)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(decodedConfig, &config)
	if err != nil {
		return err
	}
	return nil
}

func getVerifierOptions(isKeyless bool, publicKey string) *opts.VerifyOpts {
	key := "cosign.pub"
	if isKeyless && publicKey == "" {
		key = ""
		os.Setenv(integrity.ExperimentalEnv, "1")
	}

	o := &opts.VerifyOpts{
		BundlePath: "",
		VerifyOptions: co.VerifyOptions{
			Key:          key,
			CheckClaims:  true,
			Attachment:   "",
			Output:       "json",
			SignatureRef: "",
			LocalImage:   false,
			SecurityKey: co.SecurityKeyOptions{
				Use:  false,
				Slot: "",
			},
			CertVerify: co.CertVerifyOptions{
				Cert:                         "",
				CertEmail:                    "",
				CertOidcIssuer:               "",
				CertGithubWorkflowTrigger:    "",
				CertGithubWorkflowSha:        "",
				CertGithubWorkflowName:       "",
				CertGithubWorkflowRepository: "",
				CertGithubWorkflowRef:        "",
				CertChain:                    "",
				EnforceSCT:                   false,
			},
			Rekor: co.RekorOptions{URL: "https://rekor.sigstore.dev"},
			Registry: co.RegistryOptions{
				AllowInsecure:      false,
				KubernetesKeychain: false,
				RefOpts:            co.ReferenceOptions{},
				Keychain:           nil,
			},
			SignatureDigest:   co.SignatureDigestOptions{AlgorithmName: ""},
			AnnotationOptions: co.AnnotationOptions{Annotations: nil},
		},
	}
	return o
}

func extractDataFromEvent(cloudWatchEvent events.CloudwatchLogsEvent) (*FilterRecord, error) {
	b64z := cloudWatchEvent.AWSLogs.Data
	z, err := base64.StdEncoding.DecodeString(b64z)
	if err != nil {
		return nil, err
	}
	r, err := gzip.NewReader(bytes.NewReader(z))
	if err != nil {
		return nil, err
	}
	result, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	filterRecord := FilterRecord{}
	err = json.Unmarshal(result, &filterRecord)
	return &filterRecord, err
}

func main() {
	lambda.Start(HandleRequest)
}
