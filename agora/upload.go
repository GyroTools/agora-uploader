package agora

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/sirupsen/logrus"

	agoraConn "github.com/GyroTools/gtagora-connector-go/agora"
	agoraModels "github.com/GyroTools/gtagora-connector-go/agora/models"
)

type CLIProgressHandler struct {
	pw      progress.Writer
	tracker map[string]*progress.Tracker
}

func (c *CLIProgressHandler) HandleProgress(progressChan <-chan agoraModels.UploadProgress, wg *sync.WaitGroup) {
	defer wg.Done()
	for prog := range progressChan {
		if prog.Type == agoraModels.TypeUploadStarted {
			logrus.Info("Preparing Data:")
			logrus.Info("-----------------")
		}
		if prog.Type == agoraModels.TypeFileUploadStarted {
			if fileData, ok := prog.Data.(agoraModels.UploadFile); ok {
				if _, ok := c.tracker[fileData.SourcePath]; !ok {
					t := &progress.Tracker{Message: fileData.TargetPath, Total: 100, Units: progress.UnitsDefault, RemoveOnCompletion: false}
					c.tracker[fileData.SourcePath] = t
					c.pw.AppendTracker(t)
					c.pw.SetPinnedMessages("Hello World")
				}
			}
		}
		if prog.Type == agoraModels.TypeFileProgress {
			if fileProgressData, ok := prog.Data.(agoraModels.UploadProgressTransferData); ok {
				if _, ok := c.tracker[fileProgressData.File.SourcePath]; !ok {
					t := &progress.Tracker{Message: fileProgressData.File.TargetPath, Total: 100, Units: progress.UnitsDefault, RemoveOnCompletion: false}
					c.tracker[fileProgressData.File.SourcePath] = t
					c.pw.AppendTracker(t)
				}
				pct := int64(float64(fileProgressData.BytesTransfered) / float64(fileProgressData.TotalSize) * 100)
				c.tracker[fileProgressData.File.SourcePath].SetValue(pct)
			}
		}
		if prog.Type == agoraModels.TypeFileUploadCompleted {
			if fileProgressData, ok := prog.Data.(agoraModels.UploadFile); ok {
				c.tracker[fileProgressData.SourcePath].MarkAsDone()
			}
		}
	}
	for _, t := range c.tracker {
		if !t.IsDone() {
			t.MarkAsDone()
		}
	}
	// Add a small delay to allow the progress writer to update the display
	time.Sleep(100 * time.Millisecond)

	c.pw.Stop()
}

func (c *CLIProgressHandler) Cleanup() {
	c.pw.Stop()
	time.Sleep(time.Millisecond * 100)
}

func NewCLIProgressHandler(nrParallelUploads int) *CLIProgressHandler {
	var (
		flagAutoStop           = false
		flagHideETA            = true
		flagHideETAOverall     = true
		flagHideOverallTracker = false
		flagHidePercentage     = false
		flagHideTime           = true
		flagHideValue          = false
		flagShowSpeed          = false
		flagShowSpeedOverall   = false
		flagShowPinned         = false
	)

	pw := progress.NewWriter()
	pw.SetAutoStop(flagAutoStop)
	pw.SetTrackerLength(40)
	pw.SetMessageLength(30)
	pw.SetNumTrackersExpected(nrParallelUploads)
	pw.SetSortBy(progress.SortByPercentDsc)
	pw.SetStyle(progress.StyleDefault)
	pw.SetTrackerPosition(progress.PositionRight)
	pw.SetUpdateFrequency(time.Millisecond * 100)
	pw.Style().Colors = progress.StyleColorsExample
	pw.Style().Options.PercentFormat = "%4.1f%%"
	pw.Style().Visibility.ETA = !flagHideETA
	pw.Style().Visibility.ETAOverall = !flagHideETAOverall
	pw.Style().Visibility.Percentage = !flagHidePercentage
	pw.Style().Visibility.Speed = flagShowSpeed
	pw.Style().Visibility.SpeedOverall = flagShowSpeedOverall
	pw.Style().Visibility.Time = !flagHideTime
	pw.Style().Visibility.TrackerOverall = !flagHideOverallTracker
	pw.Style().Visibility.Value = !flagHideValue
	pw.Style().Visibility.Pinned = flagShowPinned

	tracker := make(map[string]*progress.Tracker)
	go pw.Render()

	return &CLIProgressHandler{pw: pw, tracker: tracker}
}

type DataFile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Sha1 string `json:"sha1"`
}

type ImportResult struct {
	DataFiles []DataFile `json:"datafiles"`
}

func sha1Hash(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	hash := h.Sum(nil)
	return hex.EncodeToString(hash[:]), nil
}

// func update_import_state(files []agoraModels.UploadFile, agora_url string, importPackageId int, api_key string) (bool, error) {
// 	logrus.Info("\nChecking Imports:")
// 	logrus.Info("-----------------")
// 	request_url := join_url(agora_url, "/api/v1/import/")
// 	request_url = join_url(request_url, fmt.Sprintf("/%d/", importPackageId))
// 	request_url = join_url(request_url, "/result/")

// 	response, err := GetRequest(request_url, api_key, "", "")
// 	if err != nil {
// 		return false, err
// 	}
// 	defer response.Body.Close()

// 	if response.StatusCode != http.StatusOK {
// 		err_status := fmt.Errorf("could not get the import result. http status = %d", response.StatusCode)
// 		return false, err_status
// 	}
// 	body, err := ioutil.ReadAll(response.Body)
// 	if err != nil {
// 		return false, err
// 	}

// 	var data []ImportResult
// 	err = json.Unmarshal(body, &data)
// 	if err != nil {
// 		return false, err
// 	}

// 	datafiles := []DataFile{}
// 	if len(data) > 0 && len(data[0].DataFiles) == 0 {
// 		return true, nil
// 	}
// 	for _, entry := range data {
// 		datafiles = append(datafiles, entry.DataFiles...)
// 	}
// 	uniqueDataFiles := make(map[int]DataFile)
// 	for _, datafile := range datafiles {
// 		uniqueDataFiles[datafile.ID] = datafile
// 	}
// 	datafiles = []DataFile{}
// 	for _, datafile := range uniqueDataFiles {
// 		datafiles = append(datafiles, datafile)
// 	}
// 	for _, datafile := range datafiles {
// 		for _, file := range files {
// 			if filepath.Base(file.TargetPath) == filepath.Base(datafile.Name) && !file.Imported {
// 				localSha1, err := sha1Hash(file.SourcePath)
// 				status := "FAILED:  "
// 				//statusColor := color.New(color.FgRed).SprintFunc()
// 				if err != nil {
// 					status = "UNKNOWN:"
// 					//statusColor = color.New(color.FgYellow).SprintFunc()
// 				}
// 				if localSha1 == datafile.Sha1 {
// 					file.Imported = true
// 					status = "IMPORTED:"
// 					//statusColor = color.New(color.FgGreen).SprintFunc()
// 				} else {
// 					fmt.Println(localSha1)
// 				}
// 				logrus.Infof("%s %s\t", status, file.SourcePath)
// 				break
// 			}
// 		}
// 	}
// 	return true, nil
// }

func Upload(agora_url string, api_key string, file_or_dir string, target_folder_id int, extract_zip bool, json_import_file string, wait bool, timeout int, verify bool, fake bool) error {
	if extract_zip {
		fileInfo, err := os.Stat(file_or_dir)
		if err == nil {
			if fileInfo.IsDir() {
				logrus.Warningf("\"--extract-zip\" has no effect when uploading a directory and will be ignored")
			} else if filepath.Ext(file_or_dir) != ".zip" {
				logrus.Warningf("no zip file found. \"--extract-zip\" will be ignored")
			}
		}
	}

	input_files := []string{file_or_dir}
	logrus.Debugf("Starting upload of %s to %s", file_or_dir, agora_url)

	agora, err := agoraConn.Create(agora_url, api_key, false)
	if err != nil {
		return err
	}

	importPackage, err := agora.NewImportPackage()
	if err != nil {
		return err
	}

	var wgUpload sync.WaitGroup
	wgUpload.Add(1)
	agoraProgressChan := make(chan agoraModels.UploadProgress)
	go func() {
		// this function receives the progress from the agora interface and passes it on to the gtPacknGo progress struct
		for prog := range agoraProgressChan {
			if prog.Type == agoraModels.TypeUploadInitialized {
				if initData, ok := prog.Data.(agoraModels.UploadProgressInitData); ok {
					logrus.Infof("Found %d files larger than %dMB which will be uploaded directly", initData.FilesToUpload, agoraModels.UPLOAD_CHUCK_SIZE/1024/1024)
					logrus.Infof("Found %d files which will be zipped and uploaded", initData.FilesToZip)
					logrus.Info("\nUploading Data:")
					logrus.Info("-----------------")

					progressHandler := NewCLIProgressHandler(initData.NrFilesToUpload)
					go progressHandler.HandleProgress(agoraProgressChan, &wgUpload)
					return
				}
			}
		}
	}()

	uploadFiles := make([]agoraModels.UploadFile, 0)
	for _, file := range input_files {
		uploadFile, err := agoraModels.NewUploadFile(file, nil)
		if err != nil {
			return err
		}
		uploadFiles = append(uploadFiles, uploadFile)
	}
	err = importPackage.Upload(uploadFiles, agoraProgressChan)
	if err != nil {
		return err
	}
	if json_import_file != "" {
		json_import_file = filepath.Base(json_import_file)
	}
	close(agoraProgressChan)
	wgUpload.Wait()
	logrus.Info("Finalizing Uploads")
	var wgComplete sync.WaitGroup
	wgComplete.Add(1)
	err = importPackage.Complete(target_folder_id, json_import_file, false, &wgComplete)
	if err != nil {
		return err
	}
	wgComplete.Wait()
	return nil
}
