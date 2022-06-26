package toolkit

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/gabriel-vasile/mimetype"
)

const randomStringSource = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0987654321_+"

// Tools is the type for the package. Create a variable of this type, and you'll have access
// to all the methods with the receiver type *Tools.
type Tools struct {
	MaxFileSize int
}

// JSONResponse is the type used for sending JSON
type JSONResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ReadJSON attempts to read the body of a request and converts it into JSON
func (t *Tools) ReadJSON(w http.ResponseWriter, r *http.Request, data any) error {
	maxBytes := 1048576 // one megabyte
	if t.MaxFileSize > 0 {
		maxBytes = t.MaxFileSize
	}

	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	err := dec.Decode(data)
	if err != nil {
		return err
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body may have only one json value")
	}

	return nil
}

// WriteJSON takes a response status code and arbitrary data and writes a json response to the client
func (t *Tools) WriteJSON(w http.ResponseWriter, status int, data any, headers ...http.Header) error {
	out, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(out)
	if err != nil {
		return err
	}

	return nil
}

// ErrorJSON takes an error, and optionally a response status code, generates and sends
// a json error response
func (t *Tools) ErrorJSON(w http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusBadRequest

	if len(status) > 0 {
		statusCode = status[0]
	}

	var payload JSONResponse
	payload.Error = true
	payload.Message = err.Error()

	return t.WriteJSON(w, statusCode, payload)
}

// RandomString returns a random string of letters of length n
func (t *Tools) RandomString(n int) string {
	s, r := make([]rune, n), []rune(randomStringSource)
	for i := range s {
		p, _ := rand.Prime(rand.Reader, len(r))
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}
	return string(s)
}

// PushJSONToRemote posts arbitrary json to an url, and returns an error,
// if any, as well as the response status code
func (t *Tools) PushJSONToRemote(client *http.Client, url string, data any) (int, error) {
	// create json we'll send
	jsonData, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return 0, err
	}

	// build the request and set header
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json")

	// call the uri
	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	return response.StatusCode, nil
}

// DownloadFile downloads a file, and attempts to force the browser to avoid displaying it
// by setting content-disposition. It also allows specification of the display name.
func (t *Tools) DownloadFile(w http.ResponseWriter, r *http.Request, p, file, displayName string) {
	fp := path.Join(p, file)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", displayName))

	http.ServeFile(w, r, fp)
}

// UploadedFile is a struct used to
type UploadedFile struct {
	NewFileName      string
	OriginalFileName string
	FileSize         int64
}

// UploadFile uploads a file to a specified directory, and gives it a random name.
// It returns the newly named file, the original file name, and a possible error.
func (t *Tools) UploadFile(r *http.Request, uploadDir string) (*UploadedFile, error) {
	// parse the form so we have access to the file
	err := r.ParseMultipartForm(1024 * 1024 * 1024)
	if err != nil {
		return nil, err
	}
	var uploadedFile UploadedFile

	for _, fHeaders := range r.MultipartForm.File {
		for _, hdr := range fHeaders {
			infile, err := hdr.Open()
			if err != nil {
				return nil, err
			}
			defer infile.Close()

			ext, err := mimetype.DetectReader(infile)
			if err != nil {
				fmt.Println(err)
				return nil, err
			}

			_, err = infile.Seek(0, 0)
			if err != nil {
				fmt.Println(err)
				return nil, err
			}

			uploadedFile.NewFileName = t.RandomString(25) + ext.Extension()
			uploadedFile.OriginalFileName = hdr.Filename

			var outfile *os.File
			defer outfile.Close()

			if outfile, err = os.Create(uploadDir + uploadedFile.NewFileName); nil != err {
				return nil, err
			} else {
				fileSize, err := io.Copy(outfile, infile)
				if err != nil {
					return nil, err
				}
				uploadedFile.FileSize = fileSize
			}
		}

	}
	return &uploadedFile, nil
}

// CreateDir creates a directory, and all necessary parent directories, if it does not exist.
func (t *Tools) CreateDir(path string) error {
	const mode = 0755
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, mode)
		if err != nil {
			return err
		}
	}
	return nil
}

//LogError checks if an error occurred and logs it
func (t *Tools) LogError(err error) {
	if err != nil {
		log.Printf("error: %v\n", err)
	}
}
