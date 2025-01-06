package main

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"agora-uploader/agora"
	"agora-uploader/log"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

var appVersion = "0.0.1"
var buildTime = "N.A."
var gitCommit = "N.A."
var gitRef = "N.A."

// generateRandomFiles generates files with random sizes in a temporary directory.
// numFiles: number of files to generate. If numFiles > 0, the total size is split into numFiles.
// totalSize: total size of all files in bytes.
func generateRandomFiles(numFiles int, totalSize int64, maxFileSize int64) (string, error) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "random_files")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %v", err)
	}

	if maxFileSize <= 0 {
		maxFileSize = totalSize
	}

	if numFiles > 0 {
		// Fixed number of files
		fileSize := totalSize / int64(numFiles)
		for i := 0; i < numFiles; i++ {
			size := fileSize
			if i == numFiles-1 {
				size = totalSize - fileSize*int64(numFiles-1)
			}
			err := createRandomFile(tempDir, size, i)
			if err != nil {
				return "", err
			}
		}
	} else {
		// Arbitrary number of files
		var generatedSize int64
		fileIndex := 0
		for generatedSize < totalSize {
			size := rand.Int63n(int64(math.Min(float64(maxFileSize), float64(totalSize-generatedSize)))) + 1
			err := createRandomFile(tempDir, size, fileIndex)
			if err != nil {
				return "", err
			}
			generatedSize += size
			fileIndex++
		}
	}

	return tempDir, nil
}

// createRandomFile creates a file with random content of the specified size.
func createRandomFile(dir string, size int64, index int) error {
	filePath := filepath.Join(dir, fmt.Sprintf("file_%d", index))
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	data := make([]byte, size)
	_, err = rand.Read(data)
	if err != nil {
		return fmt.Errorf("failed to generate random data: %v", err)
	}

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data to file: %v", err)
	}

	return nil
}

func credentials() (string, string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Agora Username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}

	fmt.Print("Agora Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", "", err
	}

	password := string(bytePassword)
	return strings.TrimSpace(username), strings.TrimSpace(password), nil
}

func getAgoraApiKey(agora_url string) string {
	user, password, _ := credentials()
	api_key := agora.GetApiKey(agora_url, user, password)
	success, err := agora.CheckConnection(agora_url, api_key)
	if !success {
		logrus.Fatal("Error: Cannot connect to the Agora server with the api-key: ", err)
	}
	return api_key
}

func Upload(c *cli.Context) error {
	if !c.IsSet("path") && !c.IsSet("test") {
		logrus.Fatal("Error: Either the --path or --test flag must be provided.")
	}

	if c.Bool("test") {
		tempDir, err := generateRandomFiles(c.Int("test-files"), int64(c.Int("test-size"))*1024*1024, int64(c.Int("test-max-file-size"))*1024*1024)
		if err != nil {
			logrus.Fatal(err)
		}
		c.Set("path", tempDir)
		defer os.RemoveAll(tempDir)
	}
	agora.HandleNoCertificateCheck(c.Bool("no-check-certificate"))
	api_key := c.String("api-key")
	if api_key == "" {
		api_key = getAgoraApiKey(c.String("url"))
	}
	err := agora.Upload(c.String("url"), api_key, c.String("path"), c.Int("target-folder"), c.Bool("extract-zip"), c.String("import-json"), true, -1, !c.Bool("no-verify"), c.Bool("fake"))
	if err != nil {
		logrus.Fatal(err)
	}
	return nil
}

func main() {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:     "url",
			Aliases:  []string{"u"},
			Value:    "",
			Usage:    "The URL to the Agora server",
			Required: true,
		},
		&cli.StringFlag{
			Name:    "path",
			Aliases: []string{"p"},
			Value:   "",
			Usage:   "The path to a file or folder to be uploaded",
		},
		&cli.IntFlag{
			Name:     "target-folder",
			Aliases:  []string{"f"},
			Value:    -1,
			Usage:    "The ID of the target folder where the data is uploaded to",
			Required: true,
		},
		&cli.StringFlag{
			Name:    "api-key",
			Aliases: []string{"k"},
			Value:   "",
			Usage:   "The Agora API key used for authentication",
		},
		&cli.BoolFlag{
			Name:  "extract-zip",
			Usage: "If the uploaded file is a zip, it is extracted and its content is imported into Agora",
		},
		&cli.BoolFlag{
			Name:  "no-verify",
			Usage: "Will not verify if all the uploaded files were correctly imported",
		},
		&cli.StringFlag{
			Name:    "import-json",
			Aliases: []string{"j"},
			Value:   "",
			Usage:   "The json which will be used for the import",
		},
		&cli.BoolFlag{
			Name:  "no-check-certificate",
			Usage: "Don't check the server certificate",
		},
		&cli.BoolFlag{
			Name:  "fake",
			Usage: "Run the uploader without actually uploading the files (for testing and debugging)",
		},
		&cli.BoolFlag{
			Name:  "test",
			Usage: "Creates random files and uploads them to the Agora server",
		},
		&cli.IntFlag{
			Name:  "test-size",
			Value: 1024,
			Usage: "The total size of the random files in megabytes",
		},
		&cli.IntFlag{
			Name:  "test-files",
			Value: -1,
			Usage: "The number of random files to generate. -1 means arbitrary number of files",
		}, &cli.IntFlag{
			Name:  "test-max-file-size",
			Value: -1,
			Usage: "The maximum size of a random file in megabytes",
		},
	}

	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s version %s\n", c.App.Name, c.App.Version)
		fmt.Printf("\nbuild time: %s\n", buildTime)
		fmt.Printf("git commit: %s\n", gitCommit)
		fmt.Printf("git ref: %s\n", gitRef)
	}

	app := &cli.App{}
	app.Name = "agora-uploader"
	app.Usage = "for uploading data to Agora"
	app.Version = appVersion
	app.Authors = []*cli.Author{
		{
			Name:  "Martin Buehrer",
			Email: "martin.buehrer@gyrotools.com",
		},
	}
	app.Flags = flags
	app.Action = Upload
	log.ConfigureLogging(app)

	err := app.Run(os.Args)
	if err != nil {
		logrus.Fatal(err)
	}
}
