# agora-uploader
helper tool for uploading data to Agora

## Download
The latest release of the agora-uploader can be found [here](https://github.com/GyroTools/agora-uploader/releases/latest/). Please make sure you choose the correct platform.

## Usage
The agora-uploader can upload a file or an entire folder from the command line with the following syntax:

```
     agora-uploader --url <agora_server_url> --path <file_or_folder> --target-folder <target_folder_id> <options>
```

```
OPTIONS:
   -u, --url              The URL of the Agora server
   -p, --path             The path to a file or folder to be uploaded
   -f, --target-folder    The ID of the target folder where the data is uploaded to (default: -1)
   -k, --api-key          The Agora API key used for authentication
   -j, --import-json      The json which will be used for the import
   -h --help              show help (default: false)
   --extract-zip          If the uploaded file is a zip, it is extracted and its content is imported into Agora (default: false)
   --no-verify            Will not verify if all the uploaded files were correctly imported (default: false)   
   --no-check-certificate Don't check the server's ssl certificate (default: false)
   --fake                 Run the uploader without actually uploading the files (for testing and debugging) (default: false)
   --test                 Creates random files and uploads them to the Agora server (default: false)
   --test-size value      The total size of the random files in megabytes (default: 1024)
   --test-files value     The number of random files to generate. -1 means arbitrary number of files (default: -1)
   --test-max-file-size   The maximum size of a random file in megabytes (default: -1)
   --verbose              Verbose mode, print processing details (default: false) 
   --log-format           Choose log format (options: runner, text, json) 
   --log-level            Log level (options: debug, info, warn, error, fatal, panic)   
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

3. Upload a folder and skip the import verification
     ```
          agora-uploader --url https://my-agora.gyrotools.com --path /data/ --target-folder 13 --no-verify
     ```

4. Upload a .zip file and import its content
     ```
          agora-uploader -u https://my-agora.gyrotools.com -p /data/my_data.zip -f 13 --extract-zip 
     ```

5. Skip the verification of the server's ssl certificate (e.g. when using a self-signed certificate)
     ```
          agora-uploader --url https://my-agora.gyrotools.com --path /data/my_dicom.dcm --target-folder 13 --no-check-certificate
     ```
