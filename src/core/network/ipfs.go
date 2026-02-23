package network

import (
	_core "YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/host"
	"YourPlace/src/core/security"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ipfsfiles "github.com/ipfs/boxo/files"
	ipfspath "github.com/ipfs/boxo/path"
	ipfscid "github.com/ipfs/go-cid"
	krpc "github.com/ipfs/kubo/client/rpc"
	kcoreifaceoptions "github.com/ipfs/kubo/core/coreiface/options"
	ma "github.com/multiformats/go-multiaddr"
)

// https://developers.cloudflare.com/distributed-web/ipfs-gateway
// var cloudflareIPFS = "https://cloudflare-ipfs.com/ipfs/%s" //CID (doesn't work with video)
// var ipfsio = "https://ipfs.io/ipfs/%s?filename=%s"         //CID & URL encoded name
// https://github.com/ipfs/kubo/tree/master/docs/examples/kubo-as-a-library
// https://github.com/empirefox/hybrid/blob/2c2a55d1c0d3a235dc7c5eea9ef430af253172e7/pkg/ipfs/migrate-directly.go
// https://github.com/zhangzhao2/idena-go/blob/151a8b1fa742d6aba28cbcd5301bece16f786ab3/ipfs/migration.go

type IPFS struct {
	rpcNode     *krpc.HttpApi
	contentPath string
	port        uint64
}

func (node *IPFS) Init(port uint64) {
	node.port = port
	UpdateIPFSConfig(port)
	if !host.RunIPFS() {
		_core.LogError("Could not run IPFS daemon")
	}
	maddr, err := ma.NewMultiaddr(strings.TrimSpace("/ip4/127.0.0.1/tcp/" + strconv.FormatUint(port, 10)))
	if err != nil {
		_core.LogError("Could not create IPFS multiaddress: " + err.Error())
		return
	}
	_node, err := krpc.NewApi(maddr)
	if err != nil {
		_core.LogError("Could not create IPFS RPC API node: " + err.Error())
		return
	}
	node.rpcNode = _node
}
func (node *IPFS) IPFSNodeAlive() bool {
	maxAttempts := 30
	sleepTime := 1 * time.Second
	attemptCount := 1
	connected := false
	for attemptCount < (maxAttempts+1) && !connected {
		addr := "127.0.0.1:" + strconv.FormatUint(node.port, 10)
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			attemptCount = attemptCount + 1
			time.Sleep(sleepTime)
			continue
		}
		err = conn.Close()
		if err != nil {
			_core.LogError("Could not close IPFS node connection: " + err.Error())
			return false
		}
		connected = true
		break
	}
	if connected {
		return true
	} else {
		_core.LogError("Could not connect to IPFS node after " + strconv.Itoa(maxAttempts) + " tries")
		return false
	}
}
func (node *IPFS) IPFSAddFile(path string) (string, error) { // Adds & pins file or directory to IPFS
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	st, err := os.Stat(path)
	if err != nil {
		return "", _core.LogErrorReturn("Could not get file stats for IPFS upload: " + err.Error())
	}
	_node, err := ipfsfiles.NewSerialFile(path, false, st)
	if err != nil {
		return "", _core.LogErrorReturn("Could not create serial file for IPFS upload: " + err.Error())
	}
	addOptions := []kcoreifaceoptions.UnixfsAddOption{
		kcoreifaceoptions.Unixfs.Pin(true, security.Hash(path)),
	}
	ipfsPath, err := node.rpcNode.Unixfs().Add(ctx, _node, addOptions...)
	if err != nil {
		return "", _core.LogErrorReturn("Could not add file to IPFS: " + err.Error())
	}
	cid := ipfsPath.RootCid().String()
	// Add to MFS (Files API) to make the file visible in the WebUI
	filename := filepath.Base(path)
	ipfsFilePath := "/ipfs/" + cid
	mfsFilePath := "/uploads/" + filename
	// Ensure the upload directory exists
	err = createMFSDirectory(node.port, "/uploads")
	if err != nil {
		return "", _core.LogErrorReturn("Could not create MFS directory: " + err.Error())
	}
	// Use HTTP API to add file to IPFS virtual file system (MFS)
	err = copyToMFS(ipfsFilePath, mfsFilePath, node.port)
	if err != nil {
		if strings.Contains(err.Error(), "directory already has entry by that name") {
			_core.LogInfo("duplicate file detected in MFS: " + path)
		} else {
			return "", _core.LogErrorReturn("Could not create MFS symlink: " + err.Error())
		}
	}
	// Verify the file was properly added and is retrievable
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_cid, err := ipfscid.Decode(cid)
	if err != nil {
		return "", _core.LogErrorReturn("Could not decode CID for file upload verification: " + err.Error())
	}
	_path := ipfspath.FromCid(_cid)
	_, err = node.rpcNode.Unixfs().Get(ctx, _path)
	if err != nil {
		return "", _core.LogErrorReturn("Could not get file from IPFS for upload verification: " + err.Error())
	}
	err = node.IPFSPinFile(cid)
	if err != nil {
		return "", _core.LogErrorReturn("Could not pin file to IPFS: " + err.Error())
	}
	return cid, nil
}
func (node *IPFS) IPFSDownloadFile(cid string, path string) error { // Downloads a file or directory from IPFS to local file system
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	exists := host.DoesExist(path)
	if exists {
		return _core.LogErrorReturn("File already exists, can't download IPFS file")
	}
	_cid, err := ipfscid.Decode(cid)
	if err != nil {
		return _core.LogErrorReturn("Could not decode CID for file download: " + err.Error())
	}
	_path := ipfspath.FromCid(_cid)
	_node, err := node.rpcNode.Unixfs().Get(ctx, _path)
	if err != nil {
		return _core.LogErrorReturn("Could not get file from IPFS: " + err.Error())
	}
	if err = ipfsfiles.WriteTo(_node, path); err != nil {
		return _core.LogErrorReturn("Could not write file to local file system: " + err.Error())
	}
	return nil

	/*resolvedPath, err := _path.NewPath(cid)
	if err != nil {
		_core.LogError("Could not get path from CID: " + err.Error())
		return err
	}
	file, err := node.rpcNode.Unixfs().Get(context.Background(), resolvedPath)
	if err != nil {
		_core.LogError("Could not get file from IPFS: " + err.Error())
		return err
	}
	return files.WriteTo(file, path)*/
}
func (node *IPFS) IPNSResolveName(ipnsName string) (string, error) {
	ctx := context.Background()
	resolved, err := node.rpcNode.Name().Resolve(ctx, ipnsName)
	if err != nil {
		return "", err
	}
	return resolved.String(), nil
}
func (node *IPFS) IPNSCreateName(cid string) (string, error) {
	ctx := context.Background()
	c, err := ipfscid.Decode(cid)
	if err != nil {
		return "", _core.LogErrorReturn("Could not decode CID: " + err.Error())
	}
	published, err := node.rpcNode.Name().Publish(ctx, ipfspath.FromCid(c))
	if err != nil {
		return "", _core.LogErrorReturn("Could not publish IPNS name: " + err.Error())
	}
	return published.String(), nil
}
func (node *IPFS) IPNSSearchName(name string) ([]string, error) {
	ctx := context.Background()
	resolved, err := node.rpcNode.Name().Search(ctx, name)
	if err != nil {
		return []string{}, err
	}
	resolvedArray := make([]string, 0)
	for result := range resolved {
		resolvedArray = append(resolvedArray, result.Path.String())
	}
	if len(resolvedArray) == 0 {
		return []string{}, _core.LogErrorReturn("Could not find any IPNS names")
	} else {
		return resolvedArray, nil
	}
}
func (node *IPFS) IPFSAddRemotePinning(name string, url string, key string) bool {
	requestString := fmt.Sprintf("http://127.0.0.1:%d/api/v0/pin/remote/service/add?arg=%s&arg=%s&arg=%s", node.port, name, url, key)
	response, err := HttpPost(requestString)
	if err != nil {
		_core.LogError("Could not add IPFS remote pinning service: " + err.Error() + " - " + response)
		return false
	}
	if strings.Contains(response, "service added") || response == "" {
		_core.LogDebug("IPFS remote pinning service '" + name + "' added successfully")
		return true
	}
	if strings.Contains(response, "service already exists") {
		_core.LogDebug("IPFS remote pinning service '" + name + "' already exists")
		return true
	}
	_core.LogError("Unexpected response when adding IPFS remote pinning service: " + response)
	return false
}
func (node *IPFS) IPFSRemoveRemotePinning(name string) bool {
	requestString := fmt.Sprintf("http://127.0.0.1:%d/api/v0/pin/remote/service/rm?arg=%s", node.port, name)
	response, err := HttpPost(requestString)
	if err != nil {
		_core.LogError("Could not remove IPFS pinning service: " + err.Error() + " - " + response)
		return false
	}
	if strings.Contains(response, "service removed") || response == "" {
		_core.LogDebug("IPFS pinning service '" + name + "' removed successfully")
		// Remove the service from the config file as well
		configPath := host.GetDataDir() + ".ipfs" + host.PathSeparator + "config"
		jsonData, err := os.ReadFile(configPath)
		if err != nil {
			_core.LogError("Could not read IPFS config file: " + err.Error())
			return true
		}
		var parsedData interface{}
		if err = json.Unmarshal(jsonData, &parsedData); err != nil {
			_core.LogError("Could not unmarshal IPFS config file: " + err.Error())
			return true
		}
		if rootMap, ok := parsedData.(map[string]interface{}); ok {
			if pinningKey, ok := rootMap["Pinning"].(map[string]interface{}); ok {
				if remoteServices, ok := pinningKey["RemoteServices"].(map[string]interface{}); ok {
					delete(remoteServices, name)
					modifiedJSON, err := json.MarshalIndent(parsedData, "", "    ")
					if err != nil {
						_core.LogError("Could not marshall IPFS config data: " + err.Error())
						return true
					}
					if err = os.WriteFile(configPath, modifiedJSON, 0644); err != nil {
						_core.LogError("Could not write to IPFS config file: " + err.Error())
						return true
					}
					_core.LogDebug("Removed '" + name + "' from IPFS config file")
				}
			}
		}
		return true
	}
	if strings.Contains(response, "service not found") {
		_core.LogDebug("IPFS pinning service '" + name + "' was not found")
		return false
	}
	_core.LogError("Unexpected response when removing IPFS pinning service: " + response)
	return false
}
func (node *IPFS) IPFSAutoAddRemotePinning(name string) bool {
	requestString := fmt.Sprintf("http://127.0.0.1:%d/api/v0/config?bool=true&arg=Pinning.RemoteServices.%s.Policies.MFS.Enable&arg=true", node.port, name)
	response, err := HttpPost(requestString)
	if err != nil {
		_core.LogError("Could not enable auto remote pinning: " + err.Error() + " - " + response)
		return false
	}
	if response == "" || strings.Contains(response, "true") {
		_core.LogDebug("Auto remote pinning enabled successfully")
		return true
	}
	_core.LogError("Unexpected response when enabling auto remote pinning: " + response)
	return false
}
func (node *IPFS) IPFSSetGateway(gateway string) bool {
	// Set the public gateway URL in IPFS config
	gatewayURL := "https://" + gateway + "/"
	requestString := fmt.Sprintf("http://127.0.0.1:%d/api/v0/config?arg=Gateway.PublicGateways.%s.Paths&arg=/ipfs&arg=/ipns&json=true", node.port, gateway)
	response, err := HttpPost(requestString)
	if err != nil {
		_core.LogError("Could not set IPFS gateway paths: " + err.Error() + " - " + response)
		return false
	}
	requestString = fmt.Sprintf("http://127.0.0.1:%d/api/v0/config?arg=Gateway.PublicGateways.%s.UseSubdomains&arg=true&bool=true", node.port, gateway)
	response, err = HttpPost(requestString)
	if err != nil {
		_core.LogError("Could not set IPFS gateway subdomains: " + err.Error() + " - " + response)
		return false
	}
	_core.LogDebug("IPFS gateway set to: " + gatewayURL)
	return true
}
func (node *IPFS) IPFSCheckPinServiceHealth(serviceName string) bool {
	listURL := fmt.Sprintf("http://127.0.0.1:%d/api/v0/pin/remote/service/ls", node.port)
	response, err := HttpGet(listURL, 10)
	if err != nil {
		_core.LogError("Failed to list pinning services: " + err.Error())
		return false
	}
	if !strings.Contains(response, serviceName) {
		_core.LogError("Pinning service '" + serviceName + "' not found in service list")
		return false
	}
	testURL := fmt.Sprintf("http://127.0.0.1:%d/api/v0/pin/remote/ls?service=%s&status=pinned&status=pinning&status=failed", node.port, serviceName)
	_, err = HttpGet(testURL, 30)
	if err != nil {
		_core.LogError("Failed to connect to pinning service '" + serviceName + "': " + err.Error())
		return false
	}
	return true
}
func (node *IPFS) IPFSPinFile(cid string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cidPath, err := ipfspath.NewPath("/ipfs/" + cid)
	if err != nil {
		return _core.LogErrorReturn("Could not create IPFS path from CID: " + err.Error())
	}
	err = node.rpcNode.Pin().Add(ctx, cidPath)
	if err != nil {
		return _core.LogErrorReturn("Could not pin file to IPFS: " + err.Error())
	}
	_core.LogDebug("Successfully pinned file locally with CID: " + cid)
	// Check if a remote pinning service is configured and pin remotely
	/*requestString := fmt.Sprintf("http://127.0.0.1:%d/api/v0/pin/remote/add?arg=%s", node.port, cid)
	response, err := HttpPost(requestString)
	if err != nil {
		_core.LogDebug("Remote pinning failed (no service configured or unreachable): " + err.Error())
		return nil
	}
	if strings.Contains(response, "error") {
		_core.LogDebug("Remote pinning failed: " + response)
		return nil
	}*/
	_core.LogDebug("Successfully pinned file remotely with CID: " + cid)
	return nil
}

func UpdateIPFSConfig(port uint64) {
	path := host.GetDataDir() + ".ipfs" + host.PathSeparator + "config"
	jsonData, err := os.ReadFile(path)
	if err != nil {
		_core.LogError("Could not read IPFS config file: " + err.Error())
		return
	}
	var parsedData interface{}
	if err = json.Unmarshal(jsonData, &parsedData); err != nil {
		_core.LogError("Could not unmarshal IPFS config file: " + err.Error())
		return
	}
	if rootMap, ok := parsedData.(map[string]interface{}); ok {
		if apiKey, _ := rootMap["API"].(map[string]interface{}); ok {
			apiKey["HTTPHeaders"].(map[string]interface{})["Access-Control-Allow-Origin"] = []string{"*"}
			apiKey["HTTPHeaders"].(map[string]interface{})["Access-Control-Allow-Methods"] = []string{"PUT", "POST", "GET", "HEAD", "OPTIONS"}
			apiKey["HTTPHeaders"].(map[string]interface{})["Access-Control-Allow-Methods"] = []string{"PUT", "POST", "GET"}
			apiKey["HTTPHeaders"].(map[string]interface{})["Cross-Origin-Resource-Policy"] = []string{"cross-origin"}
			apiKey["HTTPHeaders"].(map[string]interface{})["Cross-Origin-Embedder-Policy"] = []string{"credentialless"}
		}
		if addressesKey, _ := rootMap["Addresses"].(map[string]interface{}); ok {
			addressesKey["API"] = "/ip4/127.0.0.1/tcp/" + strconv.Itoa(int(port))
			addressesKey["Gateway"] = "/ip4/127.0.0.1/tcp/" + strconv.Itoa(int(port+1))
		}
		if gatewayKey, _ := rootMap["Gateway"].(map[string]interface{}); ok {
			gatewayKey["HTTPHeaders"].(map[string]interface{})["Access-Control-Allow-Origin"] = []string{"*"}
			gatewayKey["HTTPHeaders"].(map[string]interface{})["Access-Control-Allow-Methods"] = []string{"PUT", "POST", "GET", "HEAD", "OPTIONS"}
			gatewayKey["HTTPHeaders"].(map[string]interface{})["Cross-Origin-Resource-Policy"] = []string{"cross-origin"}
			gatewayKey["HTTPHeaders"].(map[string]interface{})["Cross-Origin-Embedder-Policy"] = []string{"credentialless"}
			localhostGateway := make(map[string]interface{})
			localhostGateway["Paths"] = []string{"/ipfs", "/ipns"}
			localhostGateway["UseSubdomains"] = true
			publicGateways := make(map[string]interface{})
			publicGateways["localhost"] = localhostGateway
			gatewayKey["PublicGateways"] = publicGateways
		}
		if swarmKey, _ := rootMap["Swarm"].(map[string]interface{}); ok {
			swarmKey["DisableNatPortMap"] = false
			swarmKey["EnableHolePunching"] = true
		}
		if routingKey, _ := rootMap["Routing"].(map[string]interface{}); ok {
			routingKey["Type"] = "auto"
			routingKey["AcceleratedDHTClient"] = false
			delete(routingKey, "Methods")
			delete(routingKey, "Routers")
		}
	}
	modifiedJSON, err := json.MarshalIndent(parsedData, "", "\t")
	if err != nil {
		_core.LogError("Could not marshall IPFS config data: " + err.Error())
		return
	}
	if err = os.WriteFile(path, modifiedJSON, 0644); err != nil {
		_core.LogError("Could not write to IPFS config file: " + err.Error())
	}
}
func createMFSDirectory(port uint64, path string) error {
	_url := fmt.Sprintf("http://localhost:%d/api/v0/files/mkdir?arg=%s&p=true", port, path)
	resp, err := http.Post(_url, "application/json", nil)
	if err != nil {
		return _core.LogErrorReturn("Could not create MFS directory: " + err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return _core.LogErrorReturn("Could not create MFS directory: " + string(body))
	}
	return nil
}
func copyToMFS(source, destination string, port uint64) error { //todo: handle existing file being reuploaded
	_url := fmt.Sprintf("http://127.0.0.1:%d/api/v0/files/cp?arg=%s&arg=%s", port, source, destination)
	resp, err := http.Post(_url, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error: %d - %s", resp.StatusCode, string(body))
	}
	return nil
}
func RestartIPFS() {
	if host.DoesProcExist("YourPlaceIpfs" + host.BinaryExtension) {
		host.KillProcess("YourPlaceIpfs" + host.BinaryExtension)
	}
	time.Sleep(3 * time.Second)
	if !host.RunIPFS() {
		_core.LogError("Could not run IPFS daemon")
	}
}

const pinataAPIURL = "https://api.pinata.cloud"
const pinataUploadsURL = "https://uploads.pinata.cloud"

type PinningService struct {
	GroupID string
	Key     string
	Type    string
	URL     string
}

func PinningServiceCreateNFTGroup(ps *PinningService) error {
	if ps.Type != "pinata" {
		return nil
	}
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", pinataAPIURL+"/v3/groups/public?name=nft&isPublic=true", nil)
	if err != nil {
		return _core.LogErrorReturn("Could not create Pinata groups request: " + err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+ps.Key)
	resp, err := client.Do(req)
	if err != nil {
		return _core.LogErrorReturn("Could not list Pinata groups: " + err.Error())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return _core.LogErrorReturn("Could not read Pinata groups response: " + err.Error())
	}
	var listResult struct {
		Data struct {
			Groups []struct {
				ID string `json:"id"`
			} `json:"groups"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &listResult); err != nil {
		return _core.LogErrorReturn("Could not parse Pinata groups response: " + err.Error())
	}
	if len(listResult.Data.Groups) > 0 {
		candidateID := listResult.Data.Groups[0].ID
		verifyReq, err := http.NewRequest("GET", pinataAPIURL+"/v3/groups/public/"+candidateID, nil)
		if err == nil {
			verifyReq.Header.Set("Authorization", "Bearer "+ps.Key)
			verifyResp, err := client.Do(verifyReq)
			if err == nil {
				verifyResp.Body.Close()
				if verifyResp.StatusCode == http.StatusOK {
					ps.GroupID = candidateID
					_core.LogDebug("Found existing Pinata NFT group: " + ps.GroupID)
					return nil
				}
			}
		}
		_core.LogDebug("Existing Pinata NFT group " + candidateID + " is invalid, creating new group")
	}
	createBody := `{"name":"nft","is_public":true}`
	req, err = http.NewRequest("POST", pinataAPIURL+"/v3/groups/public", strings.NewReader(createBody))
	if err != nil {
		return _core.LogErrorReturn("Could not create Pinata group request: " + err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+ps.Key)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		return _core.LogErrorReturn("Could not create Pinata group: " + err.Error())
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return _core.LogErrorReturn("Could not read Pinata group creation response: " + err.Error())
	}
	var createResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &createResult); err != nil {
		return _core.LogErrorReturn("Could not parse Pinata group creation response: " + err.Error())
	}
	if createResult.Data.ID == "" {
		return _core.LogErrorReturn("Pinata group creation returned empty ID")
	}
	ps.GroupID = createResult.Data.ID
	_core.LogDebug("Created Pinata NFT group: " + ps.GroupID)
	return nil
}
func PinningServiceGenerateUploadAuth(ps *PinningService) (map[string]string, error) {
	if ps.Type == "ipfs" {
		return map[string]string{
			"type":      "ipfs",
			"uploadUrl": ps.URL + "/api/v0/add",
			"key":       ps.Key,
		}, nil
	}
	now := time.Now().Unix()
	expires := int64(300)
	signBody := fmt.Sprintf(`{"date":%d,"expires":%d,"group_id":"%s"}`, now, expires, ps.GroupID)
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("POST", pinataUploadsURL+"/v3/files/sign", strings.NewReader(signBody))
	if err != nil {
		return nil, _core.LogErrorReturn("Could not create Pinata sign request: " + err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+ps.Key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, _core.LogErrorReturn("Could not get Pinata signed URL: " + err.Error())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, _core.LogErrorReturn("Could not read Pinata sign response: " + err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return nil, _core.LogErrorReturn("Pinata sign returned status " + strconv.Itoa(resp.StatusCode) + ": " + string(body))
	}
	var signResult struct {
		Data string `json:"data"`
	}
	if err = json.Unmarshal(body, &signResult); err != nil {
		return nil, _core.LogErrorReturn("Could not parse Pinata sign response: " + err.Error())
	}
	if signResult.Data == "" {
		return nil, _core.LogErrorReturn("Pinata sign returned empty URL")
	}
	return map[string]string{
		"type":      "pinata",
		"uploadUrl": signResult.Data,
		"groupId":   ps.GroupID,
	}, nil
}
func PinningServiceInit(pinningType string, pinningURL string, key string) (*PinningService, error) {
	if pinningType != "pinata" && pinningType != "ipfs" {
		return nil, _core.LogErrorReturn("Invalid pinning service type: " + pinningType + " (must be 'pinata' or 'ipfs')")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	if pinningType == "pinata" {
		req, err := http.NewRequest("GET", pinataAPIURL+"/data/testAuthentication", nil)
		if err != nil {
			return nil, _core.LogErrorReturn("Could not create Pinata auth test request: " + err.Error())
		}
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := client.Do(req)
		if err != nil {
			return nil, _core.LogErrorReturn("Pinata health check failed: " + err.Error())
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, _core.LogErrorReturn("Pinata auth test returned status " + strconv.Itoa(resp.StatusCode) + ": " + string(body))
		}
	} else {
		pinningURL = strings.TrimRight(pinningURL, "/")
		if !security.IsValidURL(pinningURL) {
			return nil, _core.LogErrorReturn("Invalid pinning service URL: " + pinningURL)
		}
		req, err := http.NewRequest("POST", pinningURL+"/api/v0/id", nil)
		if err != nil {
			return nil, _core.LogErrorReturn("Could not create IPFS node ID request: " + err.Error())
		}
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := client.Do(req)
		if err != nil {
			return nil, _core.LogErrorReturn("IPFS node health check failed: " + err.Error())
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, _core.LogErrorReturn("IPFS node ID returned status " + strconv.Itoa(resp.StatusCode))
		}
	}
	_core.LogDebug("Pinning service '" + pinningType + "' validated successfully")
	return &PinningService{
		Key:  key,
		Type: pinningType,
		URL:  pinningURL,
	}, nil
}

func UpdateBadBits(database *db.Database) {
	badbitsPath := host.GetDataDir() + ".ipfs" + host.PathSeparator + "denylists" + host.PathSeparator
	updateFlag := database.SettingsGetValue("badbitsEnabled")
	if updateFlag != "true" {
		if host.DoesExist(badbitsPath + "badbits.deny") {
			host.DeleteIfExists(badbitsPath + "badbits.deny")
			RestartIPFS()
		}
		return
	}
	badbitsURL := "https://badbits.dwebops.pub/badbits.deny"
	content, err := HttpGet(badbitsURL, 60)
	if err != nil {
		_core.LogError("Could not get bad bits list: " + err.Error())
		return
	}
	host.CreateFolder(badbitsPath)
	err = os.WriteFile(badbitsPath+"badbits.deny", []byte(content), 0644)
	if err != nil {
		_core.LogError("Could not write bad bits list: " + err.Error())
		return
	}
	RestartIPFS()
}
