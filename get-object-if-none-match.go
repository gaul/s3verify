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

// newGetObjectIfNoneMatchReq - Create a new HTTP request to perform.
func newGetObjectIfNoneMatchReq(config ServerConfig, bucketName, objectName, ETag string) (Request, error) {
	var getObjectIfNoneMatchReq = Request{
		customHeader: http.Header{},
	}

	// Set the bucketName and objectName.
	getObjectIfNoneMatchReq.bucketName = bucketName
	getObjectIfNoneMatchReq.objectName = objectName

	reader := bytes.NewReader([]byte{}) // Compute hash using empty body because GET requests do not send a body.
	_, sha256Sum, _, err := computeHash(reader)
	if err != nil {
		return Request{}, err
	}

	// Set the headers.
	getObjectIfNoneMatchReq.customHeader.Set("If-None-Match", ETag)
	getObjectIfNoneMatchReq.customHeader.Set("User-Agent", appUserAgent)
	getObjectIfNoneMatchReq.customHeader.Set("X-Amz-Content-Sha256", hex.EncodeToString(sha256Sum))

	return getObjectIfNoneMatchReq, nil
}

// getObjectIfNoneMatchVerify - Verify that the response matches with what is expected.
func getObjectIfNoneMatchVerify(res *http.Response, objectBody []byte, expectedStatusCode int) error {
	if err := verifyHeaderGetObjectIfNoneMatch(res.Header); err != nil {
		return err
	}
	if err := verifyStatusGetObjectIfNoneMatch(res.StatusCode, expectedStatusCode); err != nil {
		return err
	}
	if err := verifyBodyGetObjectIfNoneMatch(res.Body, objectBody); err != nil {
		return err
	}
	return nil
}

// verifyHeaderGetObjectIfNoneMatch - Verify that the header fields of the response match what is expected.
func verifyHeaderGetObjectIfNoneMatch(header http.Header) error {
	if err := verifyStandardHeaders(header); err != nil {
		return err
	}
	return nil
}

// verifyStatusGetObjectIfNoneMatch - Verify that the response status matches what is expected.
func verifyStatusGetObjectIfNoneMatch(respStatusCode, expectedStatusCode int) error {
	if respStatusCode != expectedStatusCode {
		err := fmt.Errorf("Unexpected Response Status Code: wanted %v, got %v", expectedStatusCode, respStatusCode)
		return err
	}
	return nil
}

// verifyBodyGetObjectIfNoneMatch - Verify that the response body matches what is expected.
func verifyBodyGetObjectIfNoneMatch(resBody io.Reader, expectedBody []byte) error {
	// The body should be returned in full.
	body, err := ioutil.ReadAll(resBody)
	if err != nil {
		return err
	}
	if !bytes.Equal(body, expectedBody) { // If the request does not go through an empty body is recieved.
		err := fmt.Errorf("Unexpected Body Recieved: wanted %v, got %v", string(expectedBody), string(body))
		return err
	}
	return nil
}

// Test the compatibility of the GetObject API when using the If-None-Match header.
func mainGetObjectIfNoneMatch(config ServerConfig, curTest int) bool {
	message := fmt.Sprintf("[%02d/%d] GetObject (If-None-Match):", curTest, globalTotalNumTest)
	// Set up an invalid ETag to test failed requests responses.
	invalidETag := "1234567890"
	bucket := validBuckets[0]
	// Spin scanBar
	scanBar(message)
	errCh := make(chan error, globalTotalNumTest)
	for _, object := range objects {
		// Spin scanBar
		scanBar(message)
		// Test with If-None-Match Header set.
		go func(objectKey, objectETag string, objectBody []byte) {
			// Create new GET object If-None-Match request.
			req, err := newGetObjectIfNoneMatchReq(config, bucket.Name, objectKey, objectETag)
			if err != nil {
				errCh <- err
				return
			}
			// Execute the request.
			res, err := config.execRequest("GET", req)
			if err != nil {
				errCh <- err
				return
			}
			defer closeResponse(res)
			// Verify the response...these checks do not check the headers yet.
			if err := getObjectIfNoneMatchVerify(res, []byte(""), 304); err != nil {
				errCh <- err
				return
			}
			// Create a bad GET object If-None-Match request with invalid ETag.
			badReq, err := newGetObjectIfNoneMatchReq(config, bucket.Name, objectKey, invalidETag)
			if err != nil {
				errCh <- err
				return
			}
			// Execute the request.
			badRes, err := config.execRequest("GET", badReq)
			if err != nil {
				errCh <- err
				return
			}
			defer closeResponse(badRes)
			// Verify the response returns the object since ETag != invalidETag
			if err := getObjectIfNoneMatchVerify(badRes, objectBody, http.StatusOK); err != nil {
				errCh <- err
				return
			}
			errCh <- nil
		}(object.Key, object.ETag, object.Body)
		// Spin scanBar
		scanBar(message)
	}
	count := len(objects)
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
	// Spin scanBar
	scanBar(message)
	// Test passed.
	printMessage(message, nil)
	return true
}
