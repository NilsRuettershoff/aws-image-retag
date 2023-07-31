package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
)

func main() {
	c := &core{}
	c.initCore()
	// get image manifest
	manifest, err := c.getImageManifest()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// retag image
	err = c.retagImage(&manifest)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("image was retagged")
}

type core struct {
	vars struct {
		profile    string
		repName    string
		currentTag string
		tag        string
	}
	clients struct {
		ecr *ecr.ECR
	}
}

func (c *core) initCore() {
	flag.StringVar(&c.vars.profile, "profile", "", "name of aws profile to use")
	flag.StringVar(&c.vars.repName, "repName", "", "name of the repository")
	flag.StringVar(&c.vars.currentTag, "currentTag", "", "the current ImageTag")
	flag.StringVar(&c.vars.tag, "tag", "", "the that should be added to image")
	flag.Parse()

	failIfNotSet(&c.vars.profile, "please provide an aws profile to use")
	failIfNotSet(&c.vars.repName, "please provide the name of ecr repository")
	failIfNotSet(&c.vars.currentTag, "please provide current tag")
	failIfNotSet(&c.vars.tag, "please provide the tag")
	// sess := session.Must(session.NewSession(&aws.Config{
	// 	Region:                        aws.String("eu-central-1"),
	// 	Credentials:                   credentials.NewSharedCredentials("", c.vars.profile),
	// 	CredentialsChainVerboseErrors: aws.Bool(true),
	// }))

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Profile: c.vars.profile,
		Config: aws.Config{
			Region: aws.String("eu-central-1"),
		},
		SharedConfigState:       session.SharedConfigEnable,
		AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
		//CredentialsChainVerboseErrors: aws.Bool(true),
	}))
	_, err := sess.Config.Credentials.Get()
	if err != nil {
		fmt.Printf("unable to find credentials: %v\n", err)
		os.Exit(1)
	}
	c.clients.ecr = ecr.New(sess)
}

func (c *core) getImageManifest() (manifest string, err error) {
	input := &ecr.BatchGetImageInput{
		ImageIds: []*ecr.ImageIdentifier{
			{
				ImageTag: aws.String(c.vars.currentTag),
			},
		},
		RepositoryName: aws.String(c.vars.repName),
	}
	result, err := c.clients.ecr.BatchGetImage(input)
	if len(result.Images) != 1 {
		fmt.Printf("%#v\n", result)
		fmt.Println(err)
		return "", errors.New("unexpected number of images")
	}
	return *result.Images[0].ImageManifest, err
}

func (c *core) retagImage(manifest *string) error {
	input := &ecr.PutImageInput{
		ImageManifest:  manifest,
		ImageTag:       aws.String(c.vars.tag),
		RepositoryName: aws.String(c.vars.repName),
	}
	_, err := c.clients.ecr.PutImage(input)
	return err
}

// failIfNotSet checks if the given argument is defined, if not it exists the programm with failmsg
func failIfNotSet(argument *string, failmsg string) {
	if argument == nil || *argument == "" {
		fmt.Println(failmsg)
		os.Exit(1)
	}
}
