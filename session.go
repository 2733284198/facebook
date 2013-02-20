// A facebook graph api client in go.
// https://github.com/huandu/facebook/
// 
// Copyright 2012, Huan Du
// Licensed under the MIT license
// https://github.com/huandu/facebook/blob/master/LICENSE

package facebook

import (
    "bytes"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
)

// Makes a facebook graph api call.
//
// Returns facebook graph api call result.
// If facebook returns error in response, returns error details in res and set err.
func (session *Session) Api(path string, method Method, params Params) (Result, error) {
    res, err := session.graph(path, method, params)

    if res != nil {
        return res, err
    }

    return nil, err
}

// Makes a batch call. Each params represent a single facebook graph api call.
// See https://developers.facebook.com/docs/reference/api/batch/ for batch call api details.
//
// Returns an array of batch call result on success.
func (session *Session) BatchApi(params ...Params) ([]Result, error) {
    return session.graphBatch(session.accessToken, params...)
}

// Gets current user id from access token.
//
// Returns error if access token is not set or invalid.
//
// It's a standard way to validate a facebook access token.
func (session *Session) User() (id string, err error) {
    if session.id != "" {
        id = session.id
        return
    }

    if session.accessToken == "" {
        err = fmt.Errorf("access token is not set.")
        return
    }

    var result Result
    result, err = session.Api("/me", GET, Params{"fields": "id"})

    if err != nil {
        return
    }

    err = result.DecodeField("id", &id)

    if err != nil {
        return
    }

    return
}

// Validates Session access token.
// Returns nil if access token is valid.
func (session *Session) Validate() (err error) {
    if session.accessToken == "" {
        err = fmt.Errorf("access token is not set.")
        return
    }

    var result Result
    result, err = session.Api("/me", GET, Params{"fields": "id"})

    if err != nil {
        return
    }

    if f := result.Get("id"); f == nil {
        err = fmt.Errorf("invalid access token.")
        return
    }

    return
}

// Gets current access token.
func (session *Session) AccessToken() string {
    return session.accessToken
}

// Sets a new access token.
func (session *Session) SetAccessToken(token string) {
    if token != session.accessToken {
        session.id = ""
        session.accessToken = token
    }
}

// Gets associated App.
func (session *Session) App() *App {
    return session.app
}

func (session *Session) graph(path string, method Method, params Params) (res Result, err error) {
    var graphUrl string
    var response []byte

    if params == nil {
        params = Params{}
    }

    // overwrite method as we always use post
    params["method"] = method

    if session.isVideoPost(path, method) {
        graphUrl = getUrl("graph_video", path, nil)
    } else {
        graphUrl = getUrl("graph", path, nil)
    }

    response, err = session.oauthRequest(graphUrl, params)

    // cannot get response from remote server
    if err != nil {
        return
    }

    err = json.Unmarshal(response, &res)

    if err != nil {
        res = nil
        err = fmt.Errorf("cannot format facebook response. %v", err)
        return
    }

    // facebook returns an error
    if _, ok := res["error"]; ok {
        err = fmt.Errorf("facebook returns an error")
    }

    return
}

func (session *Session) graphBatch(accessToken string, params ...Params) (res []Result, err error) {
    var batchParams = Params{"access_token": accessToken}
    var batchJson []byte
    var response []byte

    // encode all params to a json array.
    batchJson, err = json.Marshal(params)

    if err != nil {
        return
    }

    batchParams["batch"] = string(batchJson)

    graphUrl := getUrl("graph", "", nil)
    response, err = session.oauthRequest(graphUrl, batchParams)

    if err != nil {
        return
    }

    err = json.Unmarshal(response, &res)

    if err != nil {
        res = nil
        err = fmt.Errorf("cannot format facebook batch response. %v", err)
        return
    }

    return
}

func (session *Session) oauthRequest(url string, params Params) ([]byte, error) {
    if _, ok := params["access_token"]; !ok && session.accessToken != "" {
        params["access_token"] = session.accessToken
    }

    return session.makeRequest(url, params)
}

func (session *Session) makeRequest(url string, params Params) ([]byte, error) {
    buf := &bytes.Buffer{}
    buf.WriteString(params.Encode())
    response, err := http.Post(url, "application/x-www-form-urlencoded", buf)

    if err != nil {
        return nil, fmt.Errorf("cannot reach facebook server. %v", err)
    }

    defer response.Body.Close()

    if response.StatusCode >= 300 {
        return nil, fmt.Errorf("facebook server response an HTTP error. code: %v, body: %s",
            response.StatusCode, string(buf.Bytes()))
    }

    buf = &bytes.Buffer{}
    _, err = io.Copy(buf, response.Body)

    if err != nil {
        return nil, fmt.Errorf("cannot read facebook response. %v", err)
    }

    return buf.Bytes(), nil
}

func (session *Session) isVideoPost(path string, method Method) bool {
    return method == POST && regexpIsVideoPost.MatchString(path)
}

func getUrl(name, path string, params Params) string {
    offset := 0

    if path != "" && path[0] == '/' {
        offset = 1
    }

    buf := &bytes.Buffer{}
    buf.WriteString(domainMap[name])
    buf.WriteString(string(path[offset:]))

    if params != nil {
        buf.WriteRune('?')
        buf.WriteString(params.Encode())
    }

    return buf.String()
}

func decodeBase64URLEncodingString(data string) ([]byte, error) {
    buf := bytes.NewBufferString(data)

    // go's URLEncoding implementation requires base64 padding.
    if m := len(data) % 4; m != 0 {
        buf.WriteString(strings.Repeat("=", 4-m))
    }

    reader := base64.NewDecoder(base64.URLEncoding, buf)
    output := &bytes.Buffer{}
    _, err := io.Copy(output, reader)

    if err != nil {
        return nil, err
    }

    return output.Bytes(), nil
}
