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

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region), config.WithClientLogMode(aws.LogRequestWithBody|aws.LogSigning|aws.LogResponseWithBody))
	if err != nil {
		return err
	}

	// TODO: figure out how to do this w/o having to set
	if !debug {
		cfg.ClientLogMode.ClearRequestWithBody()
		cfg.ClientLogMode.ClearSigning()
		cfg.ClientLogMode.ClearResponseWithBody()
	}

	// Get caller identity
	identity, err := GetCallerIdentityV2(ctx, cfg)
	if err != nil {
		return fmt.Errorf("get caller identity failed: %+v", err)
	}

	// Get EC2 region
	ec2RegionResult, err := GetEc2RegionV2(ctx, cfg)
	var ec2Region string
	if err != nil {
		ec2Region = fmt.Sprintf("Does not appear to be an EC2 instance: %+v\n", err)
	} else {
		ec2Region = ec2RegionResult.Region
	}

	// Get bucket region
	bucketRegion, err := GetBucketRegionV2(ctx, cfg, bucket)
	if err != nil {
		return fmt.Errorf("get bucket region failed: %+v", err)
	}

	listObjects, err := CanListObjectsV2(ctx, cfg, bucket)
	putObject, err := CanPutObjectV2(ctx, cfg, bucket)
	getObject, err := CanGetObjectV2(ctx, cfg, bucket)

	err = cleanupLocalTestFiles()
	if err != nil {
		return fmt.Errorf("warning: failed to cleanup test files: %+v", err)
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
	PrintResult(listObjects, err, "list objects")
	PrintResult(putObject, err, "put object")
	PrintResult(getObject, err, "get object")

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
