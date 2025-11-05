package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

func isValidYourPlacePayload(payload string) (bool, int, string, map[string]interface{}) {
	var protocolRegex = regexp.MustCompile(`^yp/([\d.]+)/([a-z]+):(.+)$`)
	matches := protocolRegex.FindStringSubmatch(payload) // match the string to the protocol regex
	if matches == nil {
		core.LogDebug("Invalid YourPlace JSON payload")
		return false, 0, "", nil
	}
	versionNumber, err := strconv.Atoi(matches[1]) // get the version number
	if err != nil || !security.IsValidNumberRange(versionNumber, 1, 1) {
		core.LogDebug("Invalid YourPlace version number")
		return false, 0, "", nil
	}
	actionCode := matches[2]
	if !security.RegexMatch(`^[a-z]{1,4}$`, actionCode) {
		core.LogDebug("Invalid YourPlace action code")
		return false, 0, "", nil
	}
	var payloadObject map[string]interface{}
	err = json.Unmarshal([]byte(matches[3]), &payloadObject) // unmarshal the payload object
	if err != nil {
		core.LogDebug("Could not unmarshal YourPlace transaction payload into an object")
		return false, 0, "", nil
	}
	return true, versionNumber, actionCode, payloadObject
}
func isValidBurnAddress(blockchain string, toAddress string) bool {
	if blockchain == "base" || blockchain == "eth" {
		if toAddress != burnAddressETH {
			core.LogDebug("Metadata action not sent to burn address")
			return false
		}
		return true
	}
	return false
}
func tokenizeYourPlaceTransaction(blockchain string, transaction map[string]interface{}, timestamp uint64, blockNumber uint64) {
	// Pattern-based tokenization and database storage of YourPlace transactions
	data := transaction["input"].(string)[2:]       // get data from the transaction & drop the '0x' prefix
	decodedDataBytes, err := hex.DecodeString(data) // hex decode data
	if err != nil {
		core.LogDebug("Could not decode YourPlace transaction: " + err.Error())
		return
	}
	decodedDataStr := string(decodedDataBytes)                                                   // convert bytes to string
	isValid, versionNumber, actionCode, payloadObject := isValidYourPlacePayload(decodedDataStr) // validate transaction and parse out payload
	if !isValid {
		core.LogDebug("Could not decode YourPlace transaction: ")
		return
	}

	txHash := strings.ToLower(transaction["hash"].(string))
	fromAddress := strings.ToLower(transaction["from"].(string))
	toAddress := strings.ToLower(transaction["to"].(string))
	parentTxHash := ""
	amountHexStr := transaction["value"].(string)[2:]
	amountInt, _ := strconv.ParseUint(amountHexStr, 16, 64)
	actionPrefix := actionCode[0]   // parse out the action prefix
	actionPostfix := actionCode[1:] // parse out the action postfix

	// Execute the YourPlace transaction based on the action code
	if versionNumber == 1 {
		switch actionPrefix {
		case 'p': // Post Actions
			if toAddress != burnAddressETH {
				core.LogDebug("Post action not sent to burn address")
				return
			}
			switch actionPostfix {
			case "":
				if !handlePostTransaction(payloadObject, txHash, blockchain, fromAddress, toAddress, parentTxHash, amountInt, timestamp, blockNumber) {
					break
				}
			case "a":
				if !handlePostTransactionAttachment(payloadObject, txHash, blockchain, fromAddress, toAddress, parentTxHash, amountInt, timestamp, blockNumber) {
					break
				}
			}
			break
		case 'r': // Reply Actions
		case 'f': // Follow Actions
			switch actionPostfix {
			case "":
				blockchainPayload, ok1 := payloadObject["b"]
				addressPayload, ok2 := payloadObject["a"]
				if !ok1 || !ok2 {
					core.LogDebug("Follow action missing required fields")
					break
				}
				blockchainStr, ok1 := blockchainPayload.(string)
				addressStr, ok2 := addressPayload.(string)
				if !ok1 || !ok2 {
					core.LogDebug("Follow action fields are not strings")
					break
				}
				if !security.IsValidBlockchain(blockchainStr) {
					core.LogDebug("Invalid blockchain in follow action")
					break
				}
				if !security.IsValidAddress(addressStr, blockchainStr) {
					core.LogDebug("Invalid address in follow action")
					break
				}
				if fromAddress == addressStr && blockchain == blockchainStr { // Ignore self-follow attempts (follower count fraud)
					break
				}
				_Database.OnchainF(txHash, blockchain, fromAddress, blockchain, addressStr, blockchainStr, timestamp)
				break
			case "u":
				blockchainPayload, ok1 := payloadObject["b"]
				addressPayload, ok2 := payloadObject["a"]
				if !ok1 || !ok2 {
					core.LogDebug("Unfollow action missing required fields")
					break
				}
				blockchainStr, ok1 := blockchainPayload.(string)
				addressStr, ok2 := addressPayload.(string)
				if !ok1 || !ok2 {
					core.LogDebug("Unfollow action fields are not strings")
					break
				}
				if !security.IsValidBlockchain(blockchainStr) {
					core.LogDebug("Invalid blockchain in unfollow action")
					break
				}
				if !security.IsValidAddress(addressStr, blockchainStr) {
					core.LogDebug("Invalid address in unfollow action")
					break
				}
				if fromAddress == addressStr && blockchain == blockchainStr { // Ignore self-unfollow attempts
					break
				}
				_Database.OnchainFU(txHash, blockchain, fromAddress, blockchain, addressStr, blockchainStr, timestamp)
				break
			}
			break
		case 'm': // Metadata Actions
			if !isValidBurnAddress(blockchain, toAddress) {
				return
			}
			switch actionPostfix {
			case "n":
				name, ok1 := payloadObject["n"]
				if !ok1 {
					core.LogDebug("Metadata action missing required name field")
					break
				}
				nameStr, ok2 := name.(string)
				if !ok2 {
					core.LogDebug("Metadata action name field is not a string")
					break
				}
				nameStr = security.SanitizeNonPrintable(payloadObject["n"].(string))
				_Database.OnchainMN(blockchain, fromAddress, nameStr, timestamp)
				break
			case "a":
				avatar, ok1 := payloadObject["a"]
				if !ok1 {
					core.LogDebug("Metadata action missing required avatar field")
					break
				}
				avatarStr, ok2 := avatar.(string)
				if !ok2 {
					core.LogDebug("Metadata action avatar field is not a string")
					break
				}
				avatarStr = security.SanitizeNonPrintable(avatarStr)
				if security.IsValidURL(avatarStr) || security.IsValidCID(avatarStr) {
					_Database.OnchainMA(blockchain, fromAddress, avatarStr, timestamp)
				}
				break
			case "b":
				banner, ok1 := payloadObject["b"]
				if !ok1 {
					core.LogDebug("Metadata action missing required banner field")
					break
				}
				bannerStr, ok2 := banner.(string)
				if !ok2 {
					core.LogDebug("Metadata action banner field is not a string")
					break
				}
				bannerStr = security.SanitizeNonPrintable(bannerStr)
				if security.IsValidURL(bannerStr) || security.IsValidCID(bannerStr) {
					_Database.OnchainMB(blockchain, fromAddress, bannerStr, timestamp)
				}
				break
			case "bd":
				birthdate, ok1 := payloadObject["bd"]
				if !ok1 {
					core.LogDebug("Metadata action missing required birthdate field")
					break
				}
				birthdateStr, ok2 := birthdate.(string)
				if !ok2 {
					core.LogDebug("Metadata action birthdate field is not a string")
					break
				}
				birthdateInt, _err := strconv.ParseInt(birthdateStr, 10, 64)
				if _err != nil {
					core.LogDebug("Could not convert YourPlace transaction birthdate: " + _err.Error())
					break
				}
				if security.IsValidBirthDate(birthdateInt) {
					_Database.OnchainMBD(blockchain, fromAddress, uint64(birthdateInt), timestamp)
				}
				break
			case "l":
				location, ok1 := payloadObject["l"]
				if !ok1 {
					core.LogDebug("Metadata action missing required location field")
					break
				}
				locationStr, ok2 := location.(string)
				if !ok2 {
					core.LogDebug("Metadata action location field is not a string")
					break
				}
				locationStr = security.SanitizeNonPrintable(locationStr)
				_Database.OnchainML(blockchain, fromAddress, locationStr, timestamp)
				break
			case "w":
				website, ok1 := payloadObject["w"]
				if !ok1 {
					core.LogDebug("Metadata action missing required website field")
					break
				}
				websiteStr, ok2 := website.(string)
				if !ok2 {
					core.LogDebug("Metadata action website field is not a string")
					break
				}
				websiteStr = security.SanitizeNonPrintable(websiteStr)
				if security.IsValidURL(websiteStr) && len(websiteStr) > 0 {
					_Database.OnchainMW(blockchain, fromAddress, websiteStr, timestamp)
				}
				break
			case "d":
				description, ok1 := payloadObject["d"]
				if !ok1 {
					core.LogDebug("Metadata action missing required description field")
					break
				}
				descriptionStr, ok2 := description.(string)
				if !ok2 {
					core.LogDebug("Metadata action description field is not a string")
					break
				}
				descriptionStr = security.SanitizeNonPrintable(descriptionStr)
				if len(descriptionStr) > 0 {
					_Database.OnchainMD(blockchain, fromAddress, descriptionStr, timestamp)
				}
				break
			}
			break
		case 'b': // Blocking Actions
		case 's': // Settings Actions
		default:
			core.LogDebug("Unknown YourPlace transaction action: " + txHash)
		}
	}
}

// --- Transaction Parsing Functions --- //
func handlePostTransaction(payloadObject map[string]interface{}, txHash, blockchain, fromAddress, toAddress, parentTxHash string, amountInt uint64, timestamp uint64, blockNumber uint64) bool {
	postText, ok := payloadObject["p"]
	if !ok {
		core.LogDebug("Post Action: no p in payload")
		return false
	}
	postTextStr, ok := postText.(string)
	if !ok {
		core.LogDebug("Failed to convert post text to string")
		return false
	}
	if toAddress != "0x0" {

	}
	postTextStr = security.SanitizeNonPrintable(postTextStr)
	_Database.OnchainP(txHash, blockchain, fromAddress, toAddress, parentTxHash, amountInt, timestamp, postTextStr)
	return true
}
func handlePostTransactionAttachment(payloadObject map[string]interface{}, txHash, blockchain, fromAddress, toAddress, parentTxHash string, amountInt uint64, timestamp uint64, blockNumber uint64) bool {
	postText, ok1 := payloadObject["p"]
	attachmentsRaw, ok2 := payloadObject["a"]
	if !ok1 || !ok2 {
		core.LogDebug("Post attach action missing required fields")
		return false
	}
	postTextStr, ok3 := postText.(string)
	attachmentsArray, ok4 := attachmentsRaw.([]interface{}) // ensures array json format for the array containing all attachments
	if !ok3 || !ok4 {
		core.LogDebug("Post attach action fields are not properly typed")
		return false
	}
	parsedAttachments := []db.Attachment{}
	for _, attachment := range attachmentsArray {
		attachmentArray, ok := attachment.([]interface{}) //ensures array json format for each individual attachment
		if !ok {
			core.LogDebug("Post attach action fields are not array")
			return false
		}
		if len(attachmentArray) != 4 {
			core.LogDebug("Attachment array length is not 4")
			return false
		}
		parsedURL, okURL := attachmentArray[0].(string)
		parsedMimeType, okMimeType := attachmentArray[1].(string)
		sizeFloat, okSize := attachmentArray[2].(float64)
		fileName, okFileName := attachmentArray[3].(string)
		if !okURL || !okMimeType || !okSize || !okFileName {
			core.LogDebug("Post attach array values are not properly typed")
			return false
		}
		if !security.IsValidIndexedFilename(fileName) {
			core.LogDebug("Post attach action does not contain a valid filename")
			return false
		}
		if !security.IsValidURL(parsedURL) && !security.IsValidCID(parsedURL) {
			core.LogDebug("Post attach action does not contain a valid URL or CID")
			return false
		}
		if sizeFloat < 0 {
			core.LogDebug("Post attach action contains negative file size")
			return false
		}
		sizeUint := uint64(sizeFloat)
		parsedAttachment := db.Attachment{
			FileURL:  parsedURL,
			MimeType: parsedMimeType,
			FileSize: sizeUint,
			FileName: fileName,
		}
		parsedAttachments = append(parsedAttachments, parsedAttachment)
	}
	postTextStr = security.SanitizeNonPrintable(postTextStr)
	_Database.OnchainPA(txHash, blockchain, fromAddress, toAddress, parentTxHash, amountInt, timestamp, postTextStr, parsedAttachments)
	return true
}
