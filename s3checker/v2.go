package s3checker

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"os"
)

func CheckV2(ctx context.Context, bucket string, auth string, keyId string, accessKey string, sessionToken string, region string, debug bool) error {

	cfg, loadDefaultConfigErr := config.LoadDefaultConfig(ctx, config.WithRegion(region), config.WithClientLogMode(aws.LogRequestWithBody|aws.LogSigning|aws.LogResponseWithBody))
	if loadDefaultConfigErr != nil {
		return loadDefaultConfigErr
	}

	// TODO: figure out how to do this w/o having to set
	if !debug {
		cfg.ClientLogMode.ClearRequestWithBody()
		cfg.ClientLogMode.ClearSigning()
		cfg.ClientLogMode.ClearResponseWithBody()
	}

	// Get caller identity
	identity, getCallerIdentityErr := GetCallerIdentityV2(ctx, cfg)
	if getCallerIdentityErr != nil {
		return fmt.Errorf("get caller identity failed: %+v", getCallerIdentityErr)
	}

	// Get EC2 region
	ec2RegionResult, getEc2RegionErr := GetEc2RegionV2(ctx, cfg)
	var ec2Region string
	if getEc2RegionErr != nil {
		ec2Region = fmt.Sprintf("Does not appear to be an EC2 instance: %+v\n", getEc2RegionErr)
	} else {
		ec2Region = ec2RegionResult.Region
	}

	// Get bucket region
	bucketRegion, getBucketRegionErr := GetBucketRegionV2(ctx, cfg, bucket)
	if getBucketRegionErr != nil {
		return fmt.Errorf("get bucket region failed: %+v", getBucketRegionErr)
	}

	listObjects, canListObjectsErr := CanListObjectsV2(ctx, cfg, bucket)
	putObject, canPutObjectErr := CanPutObjectV2(ctx, cfg, bucket)
	getObject, canGetObjectErr := CanGetObjectV2(ctx, cfg, bucket)

	cleanupFilesErr := cleanupLocalTestFiles()
	if cleanupFilesErr != nil {
		return fmt.Errorf("warning: failed to cleanup test files: %+v", cleanupFilesErr)
	}

	PrintEnvVars()

	// Output caller identity
	fmt.Println("Caller identity:")
	fmt.Println(*identity.Arn)
	fmt.Println()

	// Output ec2 region
	fmt.Println("EC2 region:")
	fmt.Println(ec2Region)
	fmt.Println()

	// Output bucket region
	fmt.Println("Bucket region:")
	fmt.Println(bucketRegion)
	fmt.Println()

	fmt.Println("S3 Operations:")
	PrintResult(listObjects, canListObjectsErr, "list objects")
	PrintResult(putObject, canPutObjectErr, "put object")
	PrintResult(getObject, canGetObjectErr, "get object")

	fmt.Println("")
	fmt.Println("Access sufficient for the following CockroachDB capabilities:")
	PrintCapability(putObject && getObject && listObjects, "Backup")
	PrintCapability(getObject && listObjects, "Restore")
	PrintCapability(getObject, "Import")
	PrintCapability(putObject, "Export")
	PrintCapability(putObject, "Enterprise Changefeeds")

	return nil

}

func GetCallerIdentityV2(ctx context.Context, cfg aws.Config) (*sts.GetCallerIdentityOutput, error) {
	svc := sts.NewFromConfig(cfg)

	input := &sts.GetCallerIdentityInput{}
	return svc.GetCallerIdentity(ctx, input)
}

func GetEc2RegionV2(ctx context.Context, cfg aws.Config) (*imds.GetRegionOutput, error) {
	client := imds.NewFromConfig(cfg)
	input := &imds.GetRegionInput{}
	return client.GetRegion(ctx, input)
}

func GetBucketRegionV2(ctx context.Context, cfg aws.Config, bucket string) (string, error) {

	client := s3.NewFromConfig(cfg)
	return manager.GetBucketRegion(ctx, client, bucket)
}

func CanListObjectsV2(ctx context.Context, cfg aws.Config, bucket string) (bool, error) {
	client := s3.NewFromConfig(cfg)

	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: &bucket,
	})

	// TODO - don't iterate through all pages
	for paginator.HasMorePages() {
		_, err := paginator.NextPage(ctx)
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func CanPutObjectV2(ctx context.Context, cfg aws.Config, bucket string) (bool, error) {

	err := writeLocalTestFileForUpload(UploadTestFileLocalPath)
	if err != nil {
		return false, err
	}

	client := s3.NewFromConfig(cfg)

	localPath := UploadTestFileLocalPath
	remotePath := UploadTestFileRemotePath
	file, err := os.Open(localPath)
	if err != nil {
		fmt.Println("Unable to open file " + localPath)
		return false, err
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Printf("failed to close file: %+v\n", err)
		}
	}(file)

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		// input parameters
		Bucket: &bucket, Key: &remotePath, Body: file,
	})

	return err == nil, err
}

func CanGetObjectV2(ctx context.Context, cfg aws.Config, bucket string) (bool, error) {
	client := s3.NewFromConfig(cfg)

	file, err := os.Create(DownloadTestFileLocalPath)
	if err != nil {
		return false, err
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Printf("failed to close file: %+v\n", err)
		}
	}(file)

	downloader := manager.NewDownloader(client)
	remotePath := DownloadTestFileRemotePath
	_, err = downloader.Download(ctx, file, &s3.GetObjectInput{Bucket: &bucket, Key: &remotePath})
	return err == nil, err
}
