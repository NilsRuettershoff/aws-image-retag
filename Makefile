MAKEFLAGS=--warn-undefined-variables

SRCFOLDER := lambda
BINARY := retag
DISTFOLDER := dist
OUTPUTFOLDER := output
AWSPROFILE := default-changeme
ARTEFACTBUCKET := artefact-bucket-changeme

build: clean
	echo "Build Lambda"
	mkdir -p $(OUTPUTFOLDER)
	mkdir -p $(DISTFOLDER)
	cd $(SRCFOLDER); GOOS=linux go build -o ../$(DISTFOLDER)/$(BINARY)
	cd $(DISTFOLDER) && zip -j ../$(OUTPUTFOLDER)/retag.zip $(BINARY)
clean:
	# no refs to avoid accidential deletes
	rm -f output/*
	rm -rf dist/*
deploy: build updatelambda
updatelambda:
	aws --profile ${AWSPROFILE} lambda update-function-code --function-name retag-image --zip-file fileb://output/retag.zip
sam: build
	echo "aws sam process"
	sam deploy --profile ${AWSPROFILE} --stack-name ecr-image-retag --s3-bucket ${ARTEFACTBUCKET} --capabilities CAPABILITY_IAM

sam_wdw: build
	echo "aws sam process"
	sam deploy --profile ${AWSPROFILE} --stack-name ecr-image-retag --s3-bucket ${ARTEFACTBUCKET} --capabilities CAPABILITY_IAM --region eu-central-1
