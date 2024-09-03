package main

import (
	"bufio"
	"fmt"
	"os"
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
	agora.HandleNoCertificateCheck(c.Bool("no-check-certificate"))
	api_key := c.String("api-key")
	if api_key == "" {
		api_key = getAgoraApiKey(c.String("url"))
	}
	_, err := agora.Upload(c.String("url"), api_key, c.String("path"), c.Int("target-folder"), c.Bool("extract-zip"), c.String("import-json"), true, -1, c.Bool("verify"), c.Bool("fake"))
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
			Name:     "path",
			Aliases:  []string{"p"},
			Value:    "",
			Usage:    "The path to a file or folder to be uploaded",
			Required: true,
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
			Name:  "verify",
			Usage: "Verifies if all the uploaded files were imported correctly (waits until the import is complete)",
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
