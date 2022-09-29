package s3checker

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/sts"
	"os"
	"path/filepath"
)

const (
	SuccessPattern             = "%s -- \033[1;32msuccessful\033[0m"
	FailurePattern             = "%s -- \033[1;31mfailed with error:\033[0m\n  %+v"
	SufficientPattern          = "%s -- \033[1;32msufficient\033[0m"
	NotSufficientPattern       = "%s -- \033[1;31mnot sufficient\033[0m"
	UploadTestFileLocalPath    = "/tmp/testing1.txt"
	UploadTestFileRemotePath   = "testing1.txt"
	DownloadTestFileLocalPath  = "/tmp/testing2.txt"
	DownloadTestFileRemotePath = "testing1.txt"
	CleanupLocalTestFileGlob   = "/tmp/testing*.txt"
)

func Check(bucket string, auth string, keyId string, accessKey string, sessionToken string, region string, debug bool) error {

	sess, err := GetSession(keyId, auth, accessKey, sessionToken, region, debug)
	if err != nil {
		return fmt.Errorf("get session failed: %+v", err)
	}

	/*
		listBuckets, err := CanListBuckets(sess)
		PrintResult(listBuckets, err, "list buckets")
	*/

	fmt.Println("Caller identity:")
	identity, err := GetCallerIdentity(sess)
	if err != nil {
		return fmt.Errorf("get caller identity failed: %+v", err)
	}
	fmt.Println(identity)
	fmt.Println()

	fmt.Println("S3 Operations:")

	listObjects, err := CanListObjects(sess, bucket)
	PrintResult(listObjects, err, "list objects")

	putObject, err := CanPutObject(sess, bucket)
	PrintResult(putObject, err, "put object")

	getObject, err := CanGetObject(sess, bucket)
	PrintResult(getObject, err, "get object")

	err = cleanupLocalTestFiles()
	if err != nil {
		return fmt.Errorf("Warning: failed to cleanup test files: %+v", err)
	}

	fmt.Println("")
	fmt.Println("Access sufficient for the following CockroachDB capabilities:")
	PrintCapability(putObject && getObject && listObjects, "Backup")
	PrintCapability(getObject && listObjects, "Restore")
	PrintCapability(getObject, "Import")
	PrintCapability(putObject, "Export")
	PrintCapability(putObject, "Enterprise Changefeeds")

	return nil

}

func PrintCapability(has bool, capability string) {
	if has {
		fmt.Printf(SufficientPattern, capability)
	} else {
		fmt.Printf(NotSufficientPattern, capability)
	}
	fmt.Println()
}

func PrintResult(success bool, err error, msg string) {
	if success {
		PrintSuccess(msg)
	} else {
		PrintFailure(msg, err)
	}

}

func PrintSuccess(msg string) {
	fmt.Printf(SuccessPattern, msg)
	fmt.Println()
}

func PrintFailure(msg string, err error) {
	fmt.Printf(FailurePattern, msg, err)
	fmt.Println()
}

func GetCallerIdentity(sess *session.Session) (*sts.GetCallerIdentityOutput, error) {
	//svc := sts.New(sts.Options{Region: region})
	svc := sts.New(sess)
	input := &sts.GetCallerIdentityInput{}

	result, err := svc.GetCallerIdentity(input)
	if err != nil {
		if err2, ok := err.(awserr.Error); ok {
			switch err2.Code() {
			default:
				return result, err2
			}
		} else {
			return result, err
		}
	}

	return result, nil
}

// GetSession see https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html
func GetSession(keyId string, auth string, accessKey string, sessionToken string, region string, debug bool) (*session.Session, error) {
	if auth == "implicit" { // use implicit auth
		logLevelType := aws.LogOff
		if debug {
			logLevelType = aws.LogDebugWithRequestErrors
		}
		return session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
			Config:            aws.Config{Region: aws.String(region), LogLevel: aws.LogLevel(logLevelType)},
		})
	} else {
		return session.NewSession(&aws.Config{
			Region:      aws.String(region),
			Credentials: credentials.NewStaticCredentials(keyId, accessKey, sessionToken),
		})
	}
}

// CanListObjects List objects
func CanListObjects(sess *session.Session, bucket string) (bool, error) {
	_, err := GetObjects(sess, &bucket)
	return err == nil, err
}

/*
func PrintListObjects(sess *session.Session, bucket string) {
	resp, err := GetObjects(sess, &bucket)
	if err != nil {
		panic(err)
	}

	fmt.Println("Result of list operation:")
	for _, item := range resp.Contents {
		fmt.Println("Name:          ", *item.Key)
		fmt.Println("Last modified: ", *item.LastModified)
		fmt.Println("Size:          ", *item.Size)
		fmt.Println("Storage class: ", *item.StorageClass)
		fmt.Println("")
	}
}
*/

func GetObjects(sess *session.Session, bucket *string) (*s3.ListObjectsV2Output, error) {
	svc := s3.New(sess)

	// Get the list of items
	resp, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{Bucket: bucket})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// CanPutObject Put object
func CanPutObject(sess *session.Session, bucket string) (bool, error) {
	err := writeLocalTestFileForUpload(UploadTestFileLocalPath)
	if err != nil {
		return false, err
	}
	err = PutObject(sess, bucket, UploadTestFileLocalPath, UploadTestFileRemotePath)
	return err == nil, err
}

func PutObject(sess *session.Session, bucket string, localPath string, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		fmt.Println("Unable to open file " + localPath)
		return err
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Printf("failed to close file: %+v\n", err)
		}
	}(file)

	uploader := s3manager.NewUploader(sess)

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: &bucket,
		Key:    &remotePath,
		Body:   file,
	})
	if err != nil {
		return err
	}

	return nil
}

// CanGetObject Get object
func CanGetObject(sess *session.Session, bucket string) (bool, error) {
	// Put an object to get - let's just use the upload file to the download path
	err := PutObject(sess, bucket, UploadTestFileLocalPath, DownloadTestFileRemotePath)
	if err != nil {
		return false, err
	}
	err = DownloadObject(sess, bucket, DownloadTestFileLocalPath, DownloadTestFileRemotePath)
	return err == nil, err
}

func DownloadObject(sess *session.Session, bucket string, localPath string, remotePath string) error {
	file, err := os.Create(localPath)
	if err != nil {
		return err
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Printf("failed to close file: %+v\n", err)
		}
	}(file)

	downloader := s3manager.NewDownloader(sess)

	_, err = downloader.Download(file,
		&s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &remotePath,
		})
	if err != nil {
		return err
	}

	return nil
}

// List buckets
/*
func CanListBuckets(sess *session.Session) (bool, error) {
	_, err := GetAllBuckets(sess)
	return err == nil, err
}

func PrintAllBuckets(sess *session.Session) {
	result, err := GetAllBuckets(sess)
	if err != nil {
		panic(err)
	}

	fmt.Println("Buckets:")

	for _, bucket := range result.Buckets {
		fmt.Println(*bucket.Name + ": " + bucket.CreationDate.Format("2006-01-02 15:04:05 Monday"))
	}

}

func GetAllBuckets(sess *session.Session) (*s3.ListBucketsOutput, error) {
	svc := s3.New(sess)

	result, err := svc.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	return result, nil
}
*/

func writeLocalTestFileForUpload(path string) error {
	d := []byte("s3checker\ntest\n")
	return os.WriteFile(path, d, 0644)
}

func cleanupLocalTestFiles() error {
	files, err := filepath.Glob(CleanupLocalTestFileGlob)
	if err != nil {
		return err
	}

	for _, f := range files {
		err := os.Remove(f)
		if err != nil {
			return err
		}
	}
	return nil
}
