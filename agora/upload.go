package agora

import (
	"errors"
	"fmt"
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

func HandleResultProgress(progressChan <-chan agoraModels.UploadProgress, wg *sync.WaitGroup) {
	defer wg.Done()
	for prog := range progressChan {
		if prog.Type == agoraModels.TypeResultProgress {
			if importProgress, ok := prog.Data.(agoraModels.ResultProgress); ok {
				fmt.Printf("\rVerifying Uploaded Files: %d/%d", importProgress.NrProcessed, importProgress.NrFiles)
			}
		}
	}
	logrus.Info(" ")
}

func (c *CLIProgressHandler) HandleImportProgress(progressChan <-chan agoraModels.UploadProgress, wg *sync.WaitGroup) {
	defer wg.Done()
	for prog := range progressChan {
		if prog.Type == agoraModels.TypeImportProgress {
			if importProgress, ok := prog.Data.(agoraModels.ImportProgress); ok {
				if importProgress.State == agoraModels.STATE_ANALYZING && c.tracker["import"].Message != "Analysing Files" {
					c.tracker["import"].UpdateMessage("Analysing Files")
				} else if importProgress.State == agoraModels.STATE_IMPORTING && c.tracker["import"].Message != "Import" {
					c.tracker["import"].UpdateMessage("Import")
				}
				if importProgress.State == agoraModels.STATE_IMPORTING {
					c.tracker["import"].SetValue(int64(importProgress.Progress))
				}
			}
		}
	}
	for _, t := range c.tracker {
		if !t.IsDone() {
			t.MarkAsDone()
		}
	}
	c.Cleanup()
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
					t := &progress.Tracker{Message: filepath.Base(fileData.TargetPath), Total: fileData.Size, Units: progress.UnitsBytes, RemoveOnCompletion: true}
					c.tracker[fileData.SourcePath] = t
					c.pw.AppendTracker(t)
				}
			}
		}
		if prog.Type == agoraModels.TypeFileProgress {
			if fileProgressData, ok := prog.Data.(agoraModels.UploadProgressTransferData); ok {
				if _, ok := c.tracker[fileProgressData.File.SourcePath]; !ok {
					t := &progress.Tracker{Message: filepath.Base(fileProgressData.File.TargetPath), Total: fileProgressData.File.Size, Units: progress.UnitsBytes, RemoveOnCompletion: true}
					c.tracker[fileProgressData.File.SourcePath] = t
					c.pw.AppendTracker(t)
				}
				c.tracker[fileProgressData.File.SourcePath].SetValue(fileProgressData.BytesTransfered)
				c.tracker["total"].Increment(fileProgressData.BytesIncrement)
			}
		}
		if prog.Type == agoraModels.TypeFileUploadCompleted {
			if fileProgressData, ok := prog.Data.(agoraModels.UploadFile); ok {
				c.tracker[fileProgressData.SourcePath].MarkAsDone()
			}
		}
		if prog.Type == agoraModels.TypeImportProgress {
			if importProgressData, ok := prog.Data.(agoraModels.ImportProgress); ok {
				if _, ok := c.tracker["import"]; !ok {
					t := &progress.Tracker{Message: "Import Progress", Total: 100, Units: progress.UnitsDefault, RemoveOnCompletion: false}
					c.tracker["import"] = t
					c.pw.AppendTracker(t)
				}
				c.tracker["import"].SetValue(int64(importProgressData.Progress))
			}
		}
		if prog.Type == agoraModels.TypeUploadError {
			if fileProgressData, ok := prog.Data.(agoraModels.UploadFile); ok {
				c.tracker[fileProgressData.SourcePath].MarkAsErrored()
			}
		}
	}
	for _, t := range c.tracker {
		if !t.IsDone() {
			t.MarkAsDone()
		}
	}
	c.Cleanup()
}

func (c *CLIProgressHandler) Cleanup() {
	time.Sleep(time.Millisecond * 100)
	c.pw.Stop()
}

func NewCLIProgressHandler(nrExpectedTrackers int) *CLIProgressHandler {
	var (
		flagAutoStop           = false
		flagHideETA            = true
		flagHideETAOverall     = true
		flagHideOverallTracker = true
		flagHidePercentage     = false
		flagHideTime           = true
		flagHideValue          = false
		flagShowSpeed          = true
		flagShowSpeedOverall   = false
		flagShowPinned         = false
	)

	pw := progress.NewWriter()
	pw.SetAutoStop(flagAutoStop)
	pw.SetTrackerLength(40)
	pw.SetMessageLength(30)
	pw.SetNumTrackersExpected(nrExpectedTrackers)
	pw.SetSortBy(progress.SortByNone)
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

func prettyPrintSize(size int64) string {
	const (
		_          = iota
		KB float64 = 1 << (10 * iota)
		MB
		GB
		TB
		PB
		EB
	)

	var result string
	switch {
	case size >= int64(EB):
		result = fmt.Sprintf("%.2f EB", float64(size)/EB)
	case size >= int64(PB):
		result = fmt.Sprintf("%.2f PB", float64(size)/PB)
	case size >= int64(TB):
		result = fmt.Sprintf("%.2f TB", float64(size)/TB)
	case size >= int64(GB):
		result = fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= int64(MB):
		result = fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= int64(KB):
		result = fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		result = fmt.Sprintf("%d B", size)
	}
	return result
}

func printReport(result *agoraModels.ImportResult) {
	logrus.Info("\n\nImport Result:")
	logrus.Info("--------------")
	logrus.Infof("Total Files : %d", result.NrFiles)
	logrus.Infof("Uploaded    : %d", result.NrUploaded)
	if result.Datafiles != nil {
		logrus.Infof("Imported    : %d", result.NrImported)
		logrus.Infof("Existed     : %d", result.NrExisted)
		if result.NrIgnored > 0 {
			if result.Tasks.Error == 0 {
				logrus.Infof("\033[33;1mIgnored     : %d\033[0m", result.NrIgnored)
			} else {
				logrus.Infof("\033[31;1mFailed      : %d\033[0m", result.NrIgnored)
			}
		} else {
			logrus.Infof("Ignored     : %d", result.NrIgnored)
		}
	}

	if result.NrUploadFailed > 0 {
		logrus.Info(" ")
		logrus.Errorf("%d files failed to upload:", result.NrUploadFailed)
		for _, file := range result.UploadFailed {
			logrus.Errorf("  - %s", file)
		}
	}
	if result.NrExisted > 0 {
		logrus.Info(" ")
		logrus.Warningf("%d files were not imported because they already existed in Agora:", result.NrExisted)
		for _, file := range result.Existed {
			logrus.Warningf("  - %s", file)
		}
	}
	if result.NrIgnored > 0 {
		if result.Tasks.Error == 0 {
			logrus.Info(" ")
			logrus.Warningf("%d files were ignored during the import. This is most likely not an error", result.NrIgnored)
			for _, file := range result.Ignored {
				logrus.Warningf("  - %s", file)
			}
		} else {
			logrus.Info(" ")
			logrus.Errorf("%d files were not imported", result.NrIgnored)
			for _, file := range result.Ignored {
				logrus.Errorf("  - %s", file)
			}
		}
	}
	if result.NrHashFailed > 0 {
		logrus.Info(" ")
		logrus.Errorf("%d files were imported but the hash verification failed:", result.NrHashFailed)
		for _, file := range result.HashFailed {
			logrus.Errorf("  - %s", file)
		}
	}
	logrus.Info(" ")
	if result.NrFiles == result.NrUploaded {
		logrus.Info("\033[32mAll files were uploaded successfully!\033[0m")
	}
	if result.Datafiles != nil {
		if result.NrFiles == result.NrImported+result.NrExisted {
			logrus.Info("\033[32mAll files were imported successfully!\033[0m")
		}
	}
}

func Upload(agora_url string, api_key string, file_or_dir string, target_folder_id int, extract_zip bool, json_import_file string, wait bool, timeout int, verify bool, fake bool) error {
	if extract_zip {
		fileInfo, err := os.Stat(file_or_dir)
		if err == nil {
			if fileInfo.IsDir() {
				logrus.Warningf("\"--extract-zip\" has no effect when uploading a directory and will be ignored")
			} else if filepath.Ext(file_or_dir) != ".zip" {
				logrus.Warningf("no zip file found. \"--extract-zip\" will be ignored")
			} else if verify {
				// the import cannot be verified if a zip file is uploaded and extracted
				verify = false
			}
		}
	}
	logrus.Debugf("Starting upload of %s to %s", file_or_dir, agora_url)

	agora, err := agoraConn.Create(agora_url, api_key, false)
	if err != nil {
		return err
	}

	// check if the target folder exists
	_, err = agora.GetFolder(target_folder_id)
	if err != nil {
		return errors.New("the Agora target folder does not exist")
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
					logrus.Infof("Import Session: %d", importPackage.Id)
					logrus.Info("----------------------")
					logrus.Infof("%d files will be uploaded directly", initData.FilesToUpload)
					logrus.Infof("%d files will be zipped and uploaded", initData.FilesToZip)
					logrus.Infof("Total Size: %s", prettyPrintSize(initData.TotalSize))
					logrus.Info("\nUploading Data:")
					logrus.Info("-----------------")

					// add the total progress tracker
					progressHandler := NewCLIProgressHandler(agoraModels.PARALLEL_UPLOADS + 1)
					t := &progress.Tracker{Message: "Total", Total: initData.TotalSize, Units: progress.UnitsBytes, RemoveOnCompletion: false}
					progressHandler.tracker["total"] = t
					progressHandler.pw.AppendTracker(t)

					go progressHandler.HandleProgress(agoraProgressChan, &wgUpload)
					return
				}
			}
		}
	}()

	uploadFile, err := agoraModels.NewUploadFile(file_or_dir, nil)
	if err != nil {
		return err
	}
	err = importPackage.Upload([]agoraModels.UploadFile{uploadFile}, agoraProgressChan)
	if err != nil {
		return err
	}
	close(agoraProgressChan)
	wgUpload.Wait()
	logrus.Info("Finalizing Uploads")
	var wgComplete sync.WaitGroup
	wgComplete.Add(1)
	err = importPackage.Complete(target_folder_id, json_import_file, extract_zip, &wgComplete)
	if err != nil {
		return err
	}
	wgComplete.Wait()
	if verify {
		logrus.Info("\nWaiting for the Imports to finish...")

		importProgressChan := make(chan agoraModels.UploadProgress)
		// this wait group waits until all import progress messages are printed
		var wgImportProgress sync.WaitGroup
		wgImportProgress.Add(1)

		// create an own progress handler for the import
		importProgressHandler := NewCLIProgressHandler(1)
		t := &progress.Tracker{Message: "Import", Total: 100, Units: progress.UnitsDefault, RemoveOnCompletion: false}
		importProgressHandler.tracker["import"] = t
		importProgressHandler.pw.AppendTracker(t)
		go importProgressHandler.HandleImportProgress(importProgressChan, &wgImportProgress)

		// query the progress of the import
		err = importPackage.WaitForImport(time.Duration(30)*time.Minute, importProgressChan)
		if err != nil {
			return err
		}
		// close the progress channel and wait for the progress messages to be printed
		close(importProgressChan)
		wgImportProgress.Wait()
	}
	var wgResultProgress sync.WaitGroup
	wgResultProgress.Add(1)
	resultProgressChan := make(chan agoraModels.UploadProgress)
	go HandleResultProgress(resultProgressChan, &wgResultProgress)
	logrus.Info(" ")

	result, err := importPackage.Result(resultProgressChan)
	if err != nil {
		return err
	}
	close(resultProgressChan)
	wgResultProgress.Wait()
	printReport(result)
	return nil
}
