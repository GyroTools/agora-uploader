package agora

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var UPLOAD_CHUCK_SIZE int64 = 100 * 1024 * 1024
var MAX_ZIP_SIZE int64 = 1024 * 1024 * 1024

type ImportPackage struct {
	CompleteDate     string `json:"complete_date"`
	CreatedDate      string `json:"created_date"`
	Error            string `json:"error"`
	ExtractZipFiles  bool   `json:"extract_zip_files"`
	Id               int    `json:"id"`
	ImportFile       string `json:"import_file"`
	ImportParameters bool   `json:"import_parameters"`
	IsComplete       bool   `json:"is_complete"`
	ModifiedDate     string `json:"modified_date"`
	NofRetries       int    `json:"nof_retries"`
	State            int    `json:"state"`
	TargetId         int    `json:"target_id"`
	TargetType       int    `json:"target_type"`
	TimelineItems    []int  `json:"timeline_items"`
	User             int    `json:"user"`
}

type UploadProgressTasks struct {
	Count    int   `json:"count"`
	Finished int   `json:"finished"`
	Error    int   `json:"error"`
	Ids      []int `json:"ids"`
}

type UploadProgress struct {
	State    int                 `json:"state"`
	Progress int                 `json:"progress"`
	Tasks    UploadProgressTasks `json:"tasks"`
}

type UploadFile struct {
	SourcePath string
	TargetPath string
	Delete     bool
	Imported   bool
}

type FlowFile struct {
	State       int    `json:"state"`
	ContentHash string `json:"content_hash"`
}

type DataFile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Sha1 string `json:"sha1"`
}

type ImportResult struct {
	DataFiles []DataFile `json:"datafiles"`
}

func analyse_paths(paths []string) (files_to_upload []UploadFile, files_to_zip []UploadFile, files_only bool) {
	files_only = true
	for _, file := range paths {
		fileInfo, err := os.Stat(file)
		if err != nil {
			continue
		}
		if fileInfo.IsDir() {
			files_only = false
			filepath.Walk(file, func(path string, info os.FileInfo, err error) error {
				if !info.IsDir() {
					relative_path := path
					if strings.HasPrefix(path, file) {
						relative_path = path[len(file):]
					}
					relative_path = strings.Replace(relative_path, "\\", "/", -1)
					relative_path = strings.TrimPrefix(relative_path, "/")

					if info.Size() < UPLOAD_CHUCK_SIZE {
						files_to_zip = append(files_to_zip, UploadFile{SourcePath: strings.Replace(path, "\\", "/", -1), TargetPath: relative_path, Delete: false, Imported: false})
					} else {
						files_to_upload = append(files_to_upload, UploadFile{SourcePath: strings.Replace(path, "\\", "/", -1), TargetPath: relative_path, Delete: false, Imported: false})
					}
				}
				return nil
			})
		} else {
			abs_path, err := filepath.Abs(file)
			if err != nil {
				abs_path = file
			}
			files_to_upload = append(files_to_upload, UploadFile{SourcePath: abs_path, TargetPath: filepath.Base(file), Delete: false})
		}
	}
	return files_to_upload, files_to_zip, files_only
}

func upload_chunk(client *http.Client, url string, api_key string, values map[string]io.Reader, filename string, fake bool) (err error) {
	// Prepare a form that you will submit to that URL.
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for key, r := range values {
		var fw io.Writer
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		// Add an image file
		_, ok := r.(*bytes.Reader)
		if ok {
			if fw, err = w.CreateFormFile(key, filename); err != nil {
				return
			}
		} else {
			// Add other fields
			if fw, err = w.CreateFormField(key); err != nil {
				return
			}
		}
		if _, err = io.Copy(fw, r); err != nil {
			return err
		}

	}
	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	// Now that you have a form, you can submit it to your handler.
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return err
	}
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())
	if api_key != "" {
		req.Header.Set("Authorization", "X-Agora-Api-Key "+api_key)
	}

	// Submit the request
	if !fake {
		res, err2 := client.Do(req)
		if err2 != nil {
			return err2
		}
		// Check the response
		if res.StatusCode != http.StatusOK {
			err2 = fmt.Errorf("bad status: %s", res.Status)
			return err2
		}
	}

	return nil
}

func generateUUID() string {
	u, err := uuid.NewUUID()
	if err != nil {
		logrus.Fatal(err)
	}
	return u.String()
}

func sha256Hash(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	hash := h.Sum(nil)
	return hex.EncodeToString(hash[:]), nil
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

func verifyHash(curFile string, uid string, api_key string, uploadUrl string) (bool, error) {
	parsedURL, err := url.Parse(uploadUrl)
	if err != nil {
		return false, errors.New("error parsing URL: " + err.Error())
	}
	parsedURL.Path = fmt.Sprintf("/api/v1/flowfile/%s/", uid)
	url := parsedURL.String()

	hash_check_success := false
	hashLocal, err := sha256Hash(curFile)
	if err != nil {
		return false, err
	}
	var hashServer string

	for hashServer == "" {
		response, err := GetRequest(url, api_key, "", "")
		if err != nil {
			return false, err
		}
		defer response.Body.Close()

		if response.StatusCode == http.StatusOK {
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				return false, err
			}

			var data FlowFile
			err = json.Unmarshal(body, &data)
			if err != nil {
				return false, err
			}

			if data.State == 2 {
				hashServer = data.ContentHash
				if hashLocal != hashServer {
					continue
				} else {
					hash_check_success = true
					break
				}
			} else if data.State == 3 || data.State == 5 {
				return false, fmt.Errorf("failed to upload %v: there was an error joining the chunks", curFile)
			}
		} else {
			return false, errors.New("failed to get the hash of the file from the server")
		}
	}
	return hash_check_success, nil
}

func upload_file(request_url string, api_key string, file UploadFile, fake bool) error {
	buffer := make([]byte, UPLOAD_CHUCK_SIZE)
	logrus.Infof("Upload file: %s > %s", file.SourcePath, request_url)
	fileInfo, err := os.Stat(file.SourcePath)

	if file.Delete {
		logrus.Debugf("file will be removed after upload: %s", file.SourcePath)
		defer os.Remove(file.SourcePath)
	}

	if err != nil {
		logrus.Fatalf("Could not get the file size of %s: %v", file.SourcePath, err)
		return err
	}
	filesize := fileInfo.Size()
	nof_chunks := int(math.Ceil(float64(filesize) / float64(UPLOAD_CHUCK_SIZE)))

	client := &http.Client{}
	r, err := os.Open(file.SourcePath)
	if err != nil {
		logrus.Fatalf("Could not open the file %s: %v", file.SourcePath, err)
		return err
	}
	uuid := generateUUID()

	chunk_failed := false
	const maxRetries = 3
	for i := 0; i < nof_chunks; i++ {
		logrus.Debugf("uploading chunk %d/%d", i, nof_chunks)
		n, err := r.Read(buffer)
		if err != nil {
			chunk_failed = true
			break
		}
		chunk := bytes.NewReader(buffer[0:n])

		//prepare the reader instances to encode
		values := map[string]io.Reader{
			"file":                 chunk, // lets assume its this file
			"description":          strings.NewReader(""),
			"flowChunkNumber":      strings.NewReader(fmt.Sprintf("%d", i)),
			"flowChunkSize":        strings.NewReader(fmt.Sprintf("%d", UPLOAD_CHUCK_SIZE)),
			"flowCurrentChunkSize": strings.NewReader(fmt.Sprintf("%d", n)),
			"flowTotalSize":        strings.NewReader(fmt.Sprintf("%d", filesize)),
			"flowIdentifier":       strings.NewReader(uuid),
			"flowFilename":         strings.NewReader(file.TargetPath),
			"flowRelativePath":     strings.NewReader(file.TargetPath),
			"flowTotalChunks":      strings.NewReader(fmt.Sprintf("%d", nof_chunks)),
		}
		retries := 0
		for {
			err = upload_chunk(client, request_url, api_key, values, filepath.Base(file.SourcePath), fake)
			if err == nil {
				break
			}
			retries++
			if retries >= maxRetries {
				chunk_failed = true
				logrus.Errorf("failed to upload chunk %d/%d after %d retries", i, nof_chunks, retries)
				break
			}
			logrus.Warnf("retrying upload of chunk %d/%d (%d/%d)", i, nof_chunks, retries, maxRetries)
		}
		if chunk_failed {
			break
		}
	}
	r.Close()
	if chunk_failed {
		logrus.Errorf("could not upload the file %s", file.SourcePath)
		return err
	}

	match, err := verifyHash(file.SourcePath, uuid, api_key, request_url)
	if err != nil {
		logrus.Errorf("could not verify the hash of the file %s: %v", file.SourcePath, err)
		return err
	}
	if !match {
		err := fmt.Errorf("hashes do not match for file %s", file.SourcePath)
		logrus.Errorf("%v", err)
		return err
	}
	return nil
}

func upload_worker(fileChan chan UploadFile, request_url string, api_key string, fake bool, wg *sync.WaitGroup) {
	// Decreasing internal counter for wait-group as soon as goroutine finishes
	defer wg.Done()

	for file := range fileChan {
		upload_file(request_url, api_key, file, fake)
	}
}

func upload_files(fileCh chan UploadFile, request_url string, api_key string, files_to_upload []UploadFile, wg *sync.WaitGroup) error {
	defer wg.Done()

	// Processing all links by spreading them to `free` goroutines
	for _, file := range files_to_upload {
		fileCh <- file
	}
	return nil
}

func zip_and_upload(fileCh chan UploadFile, request_url string, api_key string, files_to_zip []UploadFile, temp_dir string, wg *sync.WaitGroup) error {
	defer wg.Done()

	index := 0
	for index < len(files_to_zip) {
		zip_filename := fmt.Sprintf("upload_%d.agora_upload", index)
		zip_path := filepath.Join(temp_dir, zip_filename)
		logrus.Debugf("creating zip file: %s", zip_path)

		file, err := os.Create(zip_path)
		if err != nil {
			logrus.Fatalf("Could not create the zip file %s: %v", zip_path, err)
			return err
		}
		defer file.Close()

		w := zip.NewWriter(file)
		defer w.Close()

		for _, file_to_zip := range files_to_zip[index:] {
			logrus.Debugf("adding file to zip: %s (path in zipfile: %s)", file_to_zip.SourcePath, file_to_zip.TargetPath)
			file, err := os.Open(file_to_zip.SourcePath)
			if err != nil {
				logrus.Fatalf("Could not open the file %s: %v", file_to_zip.SourcePath, err)
				return err
			}
			defer file.Close()

			relative_path := file_to_zip.TargetPath
			f, err := w.Create(relative_path)
			if err != nil {
				logrus.Fatalf("Could not create the path %s in zip: %v", relative_path, err)
				return err
			}

			_, err = io.Copy(f, file)
			if err != nil {
				logrus.Fatalf("Could not add %s to the zip file: %v", file_to_zip.SourcePath, err)
				return err
			}

			index += 1

			fileInfo, err := os.Stat(zip_path)
			if err == nil && fileInfo.Size() > MAX_ZIP_SIZE {
				logrus.Debugf("zip file exceeded %d MB --> uploading it", MAX_ZIP_SIZE/1024/1024)
				break
			}
		}
		w.Close()
		upload_file := UploadFile{SourcePath: zip_path, TargetPath: zip_filename, Delete: true}
		fileCh <- upload_file
	}

	return nil
}

func progress(agora_url string, api_key string, import_package_id int) (UploadProgress, error) {
	var cur_progress UploadProgress

	request_url := join_url(agora_url, "/api/v1/import/")
	request_url = join_url(request_url, fmt.Sprintf("/%d/", import_package_id))
	request_url = join_url(request_url, "/progress/") + "/"

	resp, err := GetRequest(request_url, api_key, "", "")
	if err != nil {
		return cur_progress, err
	}
	if resp.StatusCode != 200 {
		err_status := fmt.Errorf("could not get the upload progress. http status = %d", resp.StatusCode)
		return cur_progress, err_status
	}

	json.NewDecoder(resp.Body).Decode(&cur_progress)
	return cur_progress, nil
}

func complete(agora_url string, api_key string, import_package_id int, target_folder_id int, exam_id int, series_id int, task_definition_id int, json_import_file string, extract_zip_file bool) error {
	request_url := join_url(agora_url, "/api/v1/import/")
	request_url = join_url(request_url, fmt.Sprintf("/%d/", import_package_id))
	request_url = join_url(request_url, "/complete/") + "/"

	data := map[string]string{}
	if json_import_file != "" {
		data["import_file"] = json_import_file
	}
	if target_folder_id > 0 {
		data["folder"] = fmt.Sprintf("%d", target_folder_id)
	}
	if exam_id > 0 {
		data["exam"] = fmt.Sprintf("%d", exam_id)
	}
	if series_id > 0 {
		data["series"] = fmt.Sprintf("%d", series_id)
	}
	if task_definition_id > 0 {
		data["task_definition"] = fmt.Sprintf("%d", task_definition_id)
	}
	if extract_zip_file {
		data["extract_zip_files"] = "true"
	}
	json_data, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", request_url, bytes.NewBuffer(json_data))
	if err != nil {
		return err
	}
	if api_key != "" {
		req.Header.Set("Authorization", "X-Agora-Api-Key "+api_key)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 204 {
		err_status := fmt.Errorf("the \"complete\" request was invalid. http status = %d. make sure the target folder does exist", resp.StatusCode)
		return err_status
	}
	return nil
}

func get_import_package(agora_url string, api_key string) (ImportPackage, error) {
	var res ImportPackage
	request_url := join_url(agora_url, "/api/v1/import/") + "/"

	req, err := http.NewRequest("POST", request_url, nil)
	if err != nil {
		return res, err
	}

	if api_key != "" {
		req.Header.Set("Authorization", "X-Agora-Api-Key "+api_key)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return res, err
	}
	if resp.StatusCode != 201 {
		err_status := fmt.Errorf("could not get the import session. http status = %d", resp.StatusCode)
		return res, err_status
	}

	json.NewDecoder(resp.Body).Decode(&res)
	return res, nil
}

func update_import_state(files []UploadFile, agora_url string, importPackageId int, api_key string) (bool, error) {
	logrus.Info("\nChecking Imports:")
	logrus.Info("-----------------")
	request_url := join_url(agora_url, "/api/v1/import/")
	request_url = join_url(request_url, fmt.Sprintf("/%d/", importPackageId))
	request_url = join_url(request_url, "/result/")

	response, err := GetRequest(request_url, api_key, "", "")
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		err_status := fmt.Errorf("could not get the import result. http status = %d", response.StatusCode)
		return false, err_status
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return false, err
	}

	var data []ImportResult
	err = json.Unmarshal(body, &data)
	if err != nil {
		return false, err
	}

	datafiles := []DataFile{}
	if len(data) > 0 && len(data[0].DataFiles) == 0 {
		return true, nil
	}
	for _, entry := range data {
		datafiles = append(datafiles, entry.DataFiles...)
	}
	uniqueDataFiles := make(map[int]DataFile)
	for _, datafile := range datafiles {
		uniqueDataFiles[datafile.ID] = datafile
	}
	datafiles = []DataFile{}
	for _, datafile := range uniqueDataFiles {
		datafiles = append(datafiles, datafile)
	}
	for _, datafile := range datafiles {
		for _, file := range files {
			if filepath.Base(file.TargetPath) == filepath.Base(datafile.Name) && !file.Imported {
				localSha1, err := sha1Hash(file.SourcePath)
				status := "FAILED:  "
				//statusColor := color.New(color.FgRed).SprintFunc()
				if err != nil {
					status = "UNKNOWN:"
					//statusColor = color.New(color.FgYellow).SprintFunc()
				}
				if localSha1 == datafile.Sha1 {
					file.Imported = true
					status = "IMPORTED:"
					//statusColor = color.New(color.FgGreen).SprintFunc()
				}
				logrus.Infof("%s %s\t", status, file.SourcePath)
				break
			}
		}
	}
	return true, nil
}

func upload(agora_url string, api_key string, input_files []string, target_folder_id int, exam_id int, series_id int, task_definition_id int, json_import_file string, wait bool, timeout int, extract_zip bool, verify bool, fake bool) (UploadProgress, error) {
	wait = wait || verify

	logrus.Info("Preparing Data:")
	logrus.Info("-----------------")
	files_to_upload, files_to_zip, files_only := analyse_paths(input_files)
	if !files_only {
		logrus.Infof("Found %d files larger than %dMB which will be uploaded directly", len(files_to_upload), UPLOAD_CHUCK_SIZE/1024/1024)
		logrus.Infof("Found %d files which will be zipped and uploaded", len(files_to_zip))
	}
	allFiles := append(files_to_upload, files_to_zip...)
	logrus.Info("\nUploading Data:")
	logrus.Info("-----------------")

	import_package, err := get_import_package(agora_url, api_key)
	if err != nil {
		return UploadProgress{}, err
	}

	request_url := join_url(agora_url, "/api/v1/import/")
	request_url = join_url(request_url, fmt.Sprintf("/%d/", import_package.Id))
	request_url = join_url(request_url, "/upload/") + "/"

	// we have 2 threadpools here. One performs the large file upload and the zipping in parallel. One performs a parallel file upload
	parallel_uploads := 3

	fileCh := make(chan UploadFile)
	wg := new(sync.WaitGroup)

	// Adding routines to workgroup and running then
	for i := 0; i < parallel_uploads; i++ {
		wg.Add(1)
		go upload_worker(fileCh, request_url, api_key, fake, wg)
	}

	temp_dir, err := ioutil.TempDir("", "agora_app")
	defer os.RemoveAll(temp_dir)
	if err != nil {
		return UploadProgress{}, err
	}

	wg_upload_zip := new(sync.WaitGroup)
	wg_upload_zip.Add(2)

	go upload_files(fileCh, request_url, api_key, files_to_upload, wg_upload_zip)
	go zip_and_upload(fileCh, request_url, api_key, files_to_zip, temp_dir, wg_upload_zip)
	wg_upload_zip.Wait()

	// Closing channel (waiting in goroutines won't continue any more)
	close(fileCh)

	// Waiting for all goroutines to finish (otherwise they die as main routine dies)
	wg.Wait()

	if err = complete(agora_url, api_key, import_package.Id, target_folder_id, exam_id, series_id, task_definition_id, json_import_file, extract_zip); err == nil {
		if wait {
			if verify {
				logrus.Info("\nWaiting for the Imports to finish...")
			} else {
				logrus.Info("\nWaiting for the Uploads to finish...")
			}
			start_time := time.Now()
			for timeout < 0 || time.Since(start_time).Seconds() < float64(timeout) {
				data, err := progress(agora_url, api_key, import_package.Id)
				if err != nil {
					return UploadProgress{}, err
				}
				if data.State == 5 || data.State == 4 {
					if verify {

						if data.State == 5 && data.Progress == 100 {
							success, err := update_import_state(allFiles, agora_url, import_package.Id, api_key)
							if err != nil {
								return UploadProgress{}, err
							}
							if success {
								logrus.Info("\nAll files were imported successfully!\n")
								return data, nil
							} else {
								logrus.Error("\nNot all files were imported successfully!\n")
								return data, nil
							}
						}
					} else {
						return data, nil
					}

				} else if data.State == -1 {
					return data, errors.New("the import failed")
				}
				time.Sleep(5 * time.Second)
			}
			err = errors.New("upload progress timeout")
			return UploadProgress{}, err
		}
	} else {
		return UploadProgress{}, err
	}
	return UploadProgress{}, nil
}

func Upload(agora_url string, api_key string, file_or_dir string, target_folder_id int, extract_zip bool, json_import_file string, wait bool, timeout int, verify bool, fake bool) (UploadProgress, error) {
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
	return upload(agora_url, api_key, input_files, target_folder_id, -1, -1, -1, json_import_file, wait, timeout, extract_zip, verify, fake)
}
