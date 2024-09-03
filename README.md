# agora-uploader
helper tool for uploading data to Agora

## Download
The latest release of the agora-uploader can be found [here](https://github.com/GyroTools/agora-uploader/releases/latest/). Please make sure you choose the correct platform.

## Usage
The agora-uploader can upload a file or an entire folder from the command line with the following syntax:

```
     agora-uploader --path <file_or_folder> <options>
```

```
OPTIONS:
   -u, --url              The URL of the Agora server
   -p, --path             The path to a file or folder to be uploaded
   -f, --target-folder    The ID of the target folder where the data is uploaded to (default: -1)
   -k, --api-key          The Agora API key used for authentication 
   --verify               Verifies if all the uploaded files were imported correctly (waits until the import is complete)
   --extract-zip          If the uploaded file is a zip, it is extracted and its content is imported into Agora (default: false)   
   --no-check-certificate Don't check the server certificate
   --import-json          The json which will be used for the import 
   --fake                 Run the uploader without actually uploading the files (for testing and debugging) (default: false)
   --help                 show help (default: false)
```

### Examples

1. Upload a file into the Agora folder with ID = 13 (username and password are promted on the commandline)
     ```
          agora-uploader --url https://my-agora.gyrotools.com --path /data/my_dicom.dcm --target-folder 13
     ```

2. Upload an entire folder and use the Agora api-key for authentication
     ```
          agora-uploader -u https://my-agora.gyrotools.com -p /data/ -f 13 --api-key 8be8b7bd-5007-4af9-95fa-4c491566d40a
     ```

3. Upload a folder and verify that all uploaded files have been imported
     ```
          agora-uploader --url https://my-agora.gyrotools.com --path /data/ --target-folder 13 --verify
     ```

4. Upload a .zip file and import its content
     ```
          agora-uploader -u https://my-agora.gyrotools.com -p /data/my_data.zip -f 13 --extract-zip 
     ```

5. Skip the verification of the server's ssl certificate (e.g. when using a self-signed certificate)
     ```
          agora-uploader --url https://my-agora.gyrotools.com --path /data/my_dicom.dcm --target-folder 13 --no-check-certificate
     ```
