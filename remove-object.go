/*
 * Minio S3Verify Library for Amazon S3 Compatible Cloud Storage (C) 2016 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

// newRemoveObjectReq - Create a new DELETE object HTTP request.
func newRemoveObjectReq(config ServerConfig, bucketName, objectName string) (Request, error) {
	var removeObjectReq = Request{
		customHeader: http.Header{},
	}

	// Set the bucketName and objectName.
	removeObjectReq.bucketName = bucketName
	removeObjectReq.objectName = objectName

	reader := bytes.NewReader([]byte{}) // Compute hash using empty body because DELETE requests do not send a body.
	_, sha256Sum, _, err := computeHash(reader)
	if err != nil {
		return Request{}, err
	}

	// Set the headers.
	removeObjectReq.customHeader.Set("User-Agent", appUserAgent)
	removeObjectReq.customHeader.Set("X-Amz-Content-Sha256", hex.EncodeToString(sha256Sum))

	return removeObjectReq, nil
}

// removeObjectVerify - Verify that the response returned matches what is expected.
func removeObjectVerify(res *http.Response, expectedStatusCode int) error {
	if err := verifyHeaderRemoveObject(res.Header); err != nil {
		return err
	}
	if err := verifyBodyRemoveObject(res.Body); err != nil {
		return err
	}
	if err := verifyStatusRemoveObject(res.StatusCode, expectedStatusCode); err != nil {
		return err
	}
	return nil
}

// verifyHeaderRemoveObject - Verify that header returned matches what is expected.
func verifyHeaderRemoveObject(header http.Header) error {
	if err := verifyStandardHeaders(header); err != nil {
		return err
	}
	return nil
}

// verifyBodyRemoveObject - Verify that the body returned is empty.
func verifyBodyRemoveObject(resBody io.Reader) error {
	body, err := ioutil.ReadAll(resBody)
	if err != nil {
		return err
	}
	if !bytes.Equal(body, []byte{}) {
		err := fmt.Errorf("Unexpected Body Received: %v", string(body))
		return err
	}
	return nil
}

// verifyStatusRemoveObject - Verify that the status returned matches what is expected.
func verifyStatusRemoveObject(respStatusCode, expectedStatusCode int) error {
	if respStatusCode != expectedStatusCode {
		err := fmt.Errorf("Unexpected Status Received: wanted %d, got %d", expectedStatusCode, respStatusCode)
		return err
	}
	return nil
}

// mainRemoveObjectExists - Entry point for the RemoveObject API test when object exists.
func mainRemoveObjectExists(config ServerConfig, curTest int) bool {
	message := fmt.Sprintf("[%d/%d] RemoveObject:", curTest, globalTotalNumTest)
	errCh := make(chan error, globalTotalNumTest)
	// Spin scanBar
	scanBar(message)
	for _, bucket := range validBuckets {
		for _, object := range objects {
			// Spin scanBar
			scanBar(message)
			go func(bucketName, objectKey string) {
				// Create a new request.
				req, err := newRemoveObjectReq(config, bucketName, objectKey)
				if err != nil {
					errCh <- err
					return
				}
				// Execute the request.
				res, err := config.execRequest("DELETE", req)
				if err != nil {
					errCh <- err
					return
				}
				defer closeResponse(res)
				// Verify the response.
				if err := removeObjectVerify(res, http.StatusNoContent); err != nil {
					errCh <- err
					return
				}
				errCh <- nil
			}(bucket.Name, object.Key)
			// Spin scanBar
			scanBar(message)

		}
		count := len(objects)
		for count > 0 {
			count--
			err, ok := <-errCh
			if !ok {
				return false
			}
			if err != nil {
				printMessage(message, err)
				return false
			}
			// Spin scanBar
			scanBar(message)
		}
		for _, object := range copyObjects {
			// Spin scanBar
			scanBar(message)
			go func(bucketName, objectKey string) {
				// Create a new request.
				req, err := newRemoveObjectReq(config, bucketName, objectKey)
				if err != nil {
					errCh <- err
					return
				}
				// Execute the request.
				res, err := config.execRequest("DELETE", req)
				if err != nil {
					errCh <- err
					return
				}
				defer closeResponse(res)
				// Verify the response.
				if err := removeObjectVerify(res, http.StatusNoContent); err != nil {
					errCh <- err
					return
				}
				errCh <- nil
			}(bucket.Name, object.Key)
			// Spin scanBar
			scanBar(message)
		}
		count = len(copyObjects)
		for count > 0 {
			count--
			// Spin scanBar
			scanBar(message)
			err, ok := <-errCh
			if !ok {
				return false
			}
			if err != nil {
				printMessage(message, err)
				return false
			}
			// Spin scanBar
			scanBar(message)
		}
		for _, object := range multipartObjects {
			// Spin scanBar
			scanBar(message)
			go func(bucketName, objectKey string) {
				// Create a new request.
				req, err := newRemoveObjectReq(config, bucketName, objectKey)
				if err != nil {
					errCh <- err
					return
				}
				// Execute the request.
				res, err := config.execRequest("DELETE", req)
				if err != nil {
					errCh <- err
					return
				}
				defer closeResponse(res)
				// Verify the response.
				if err := removeObjectVerify(res, http.StatusNoContent); err != nil {
					errCh <- err
					return
				}
				errCh <- nil
			}(bucket.Name, object.Key)
			// Spin scanBar
			scanBar(message)
		}
		count = len(multipartObjects)
		for count > 0 {
			count--
			// Spin scanBar
			scanBar(message)
			err, ok := <-errCh
			if !ok {
				return false
			}
			if err != nil {
				printMessage(message, err)
				return false
			}
			// Spin scanBar
			scanBar(message)
		}
	}
	// Spin scanBar
	scanBar(message)
	// Test passed.
	printMessage(message, nil)
	return true
}
