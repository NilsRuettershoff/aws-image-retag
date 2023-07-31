package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codepipeline"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/google/uuid"
)

var (
	ecrClient *ecr.ECR
	cpClient  *codepipeline.CodePipeline
)

func init() {
	sess := session.Must(session.NewSession(&aws.Config{
		Region:                        aws.String("eu-central-1"),
		CredentialsChainVerboseErrors: aws.Bool(true),
	}))
	_, err := sess.Config.Credentials.Get()
	if err != nil {
		fmt.Printf("unable to find credentials: %v\n", err)
		os.Exit(1)
	}
	ecrClient = ecr.New(sess)
	cpClient = codepipeline.New(sess)
}

func main() {
	lambda.Start(handleRetag)
}

type workParameters struct {
	RepName   string `json:"rep_name"`
	CommitTag string `json:"commit_tag"`
	SlimTag   string // created during processing
	NewTag    string `json:"new_tag"`
	JobID     string // enriched from input request
}

func (wp *workParameters) validate() {
	failIfNotSet(&wp.RepName, "repository name was not provided", wp)
	failIfNotSet(&wp.NewTag, "the new tag was not provided", wp)
	failIfNotSet(&wp.CommitTag, "the full commit hash was not provided", wp)
}

func (wp *workParameters) extractShotTag() {
	wp.SlimTag = wp.CommitTag[0:8]
}

// handleRetag is the lambda handler
func handleRetag(ctx context.Context, req events.CodePipelineEvent) {
	wp := workParameters{}
	wp.JobID = req.CodePipelineJob.ID
	userParameters := req.CodePipelineJob.Data.ActionConfiguration.Configuration.UserParameters
	fmt.Printf("UserParameters: %s", userParameters)
	err := json.Unmarshal([]byte(userParameters), &wp)
	if err != nil {
		message := fmt.Sprintf("unable to unmarshal user parameters: %v", err)
		fmt.Println(message)
		reportFailure(wp.JobID, message)
		os.Exit(1)
	}
	wp.validate()
	wp.extractShotTag()
	manifest, err := getImageManifest(&wp)
	if err != nil {
		message := fmt.Sprintf("unable to retieve manifest %#v", err)
		fmt.Println(message)
		reportFailure(wp.JobID, message)

		os.Exit(1)
	}
	err = retagImage(&manifest, &wp)
	if err != nil {
		message := fmt.Sprintf("unable to retag image %#v", err)
		fmt.Println(message)
		reportFailure(wp.JobID, message)
		os.Exit(1)
	}
	reportSuccess(wp.JobID)

}

func getImageManifest(wp *workParameters) (manifest string, err error) {
	input := &ecr.BatchGetImageInput{
		ImageIds: []*ecr.ImageIdentifier{
			{
				ImageTag: aws.String(wp.SlimTag),
			},
		},
		RepositoryName: aws.String(wp.RepName),
	}
	result, err := ecrClient.BatchGetImage(input)
	if len(result.Images) != 1 {
		fmt.Printf("%#v\n", result)
		fmt.Println(err)
		reportFailure(wp.JobID, "error during image fetching")
		return "", errors.New("unexpected number of images")
	}
	return *result.Images[0].ImageManifest, err
}

func retagImage(manifest *string, wp *workParameters) error {
	input := &ecr.PutImageInput{
		ImageManifest:  manifest,
		ImageTag:       aws.String(wp.NewTag),
		RepositoryName: aws.String(wp.RepName),
	}
	_, err := ecrClient.PutImage(input)
	return err
}

// failIfNotSet checks if the given argument is defined, if not it exists the programm with failmsg
func failIfNotSet(argument *string, failmsg string, wp *workParameters) {
	if argument == nil || *argument == "" {
		fmt.Println(failmsg)
		reportFailure(wp.JobID, failmsg)
		os.Exit(1)
	}
}

func reportSuccess(jobid string) {
	input := codepipeline.PutJobSuccessResultInput{
		JobId: aws.String(jobid),
		ExecutionDetails: &codepipeline.ExecutionDetails{
			ExternalExecutionId: aws.String(uuid.New().String()),
			Summary:             aws.String("image was sucessfull retagged"),
			PercentComplete:     aws.Int64(100),
		},
	}
	_, err := cpClient.PutJobSuccessResult(&input)
	if err != nil {
		// try to report failure
		reportFailure(jobid, "could not report success")
		fmt.Printf("unable to report success: %#v", err)
	}
}

func reportFailure(jobid string, errmessage string) {
	input := codepipeline.PutJobFailureResultInput{
		JobId: aws.String(jobid),
		FailureDetails: &codepipeline.FailureDetails{
			ExternalExecutionId: aws.String(uuid.New().String()),
			Message:             aws.String(errmessage),
			Type:                aws.String("JobFailed"),
		},
	}
	_, err := cpClient.PutJobFailureResult(&input)
	if err != nil {
		fmt.Printf("unable to report success: %#v", err)
	}
}
