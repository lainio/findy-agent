package comm

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/findy-network/findy-agent/agent/didcomm"
	"github.com/findy-network/findy-agent/agent/endp"
	"github.com/findy-network/findy-agent/agent/sec"
	"github.com/findy-network/findy-agent/agent/utils"
	"github.com/golang/glog"
	"github.com/lainio/err2"
	"github.com/lainio/err2/try"
)

// errorMessageMaxLength is the maximum length of the response body we will
// include into the generated error message
const errorMessageMaxLength = 80

// SendAndWaitReq is proxy function to route actual call to http or pseudo http in tests.
var SendAndWaitReq = sendAndWaitHTTPRequest

// FileDownload is proxy function to route actual call to http or pseudo http in tests.
var FileDownload = downloadFile

func sendAndWaitHTTPRequest(urlStr string, msg io.Reader, timeout time.Duration) (data []byte, err error) {
	defer err2.Annotate("call http", &err)

	c := &http.Client{
		Timeout: timeout,
	}
	URL := try.To1(url.Parse(urlStr))

	glog.V(1).Infof("Posting message to %s\n", urlStr)

	request, _ := http.NewRequest("POST", URL.String(), msg)

	// TODO: make configurable when there is support for application/didcomm-envelope-enc
	request.Header.Set("Content-Type", "application/ssi-agent-wire")

	response := try.To1(c.Do(request))

	defer func() {
		_ = response.Body.Close()
	}()

	data, err = ioutil.ReadAll(response.Body)

	return checkHTTPStatus(response, data)
}

// checkHTTPStatus checks the status code and gets the server message
func checkHTTPStatus(response *http.Response, data []byte) ([]byte, error) {
	if response.StatusCode != http.StatusOK {
		glog.Warning("http code:", response.Status)
		contentType := response.Header.Get("Content-type")
		// from our server: text/plain; charset=utf-8
		if strings.HasPrefix(contentType, "text/plain") {
			l := len(data)
			return nil, fmt.Errorf("%s: %s",
				response.Status, data[0:min(errorMessageMaxLength, l)])
		}
		return nil, fmt.Errorf("%v",
			response.Status)
	}
	return data, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// downloadFile from url and sets the filepath to name of the local file.
// If filepath is empty uses the filename of the download URL.
func downloadFile(downloadDir, filepath, url string) (name string, err error) {
	defer err2.Annotate("download file", &err)

	// Get the data stream from server
	resp := try.To1(http.Get(url))

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download file: %s", resp.Status)
	}

	filename := filepath
	if filename == "" {
		filename = path.Base(resp.Request.URL.String())
	}
	filename = path.Join(downloadDir, filename)
	out := try.To1(os.Create(filename))
	defer func() {
		_ = resp.Body.Close()
		_ = out.Close()
	}()

	// Stream copy, can be used for large files as well
	try.To1(io.Copy(out, resp.Body))

	return filename, nil
}

/*
SendPL is helper function to send a protocol messages to receiver which is
defined in the Task.ReceiverEndp. Function will encrypt messages before sending.
It doesn't resend PL in case of failure. The recovering in done at PSM level.
*/
func SendPL(sendPipe sec.Pipe, task Task, opl didcomm.Payload) (err error) {
	defer err2.Annotate("send payload", &err)

	cnxAddr := endp.NewAddrFromPublic(task.ReceiverEndp())

	cryptSendPL, _ := try.To2(sendPipe.Pack(opl.JSON()))

	_, err = SendAndWaitReq(cnxAddr.Address(), bytes.NewReader(cryptSendPL),
		utils.Settings.Timeout())
	return err
}
