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
		core.LogDebug("Invalid YourPlace JSON payload: " + payload)
		return false, 0, "", nil
	}
	versionNumber, err := strconv.Atoi(matches[1]) // get the version number
	if err != nil || !security.IsValidNumberRange(versionNumber, 1, 1) {
		core.LogDebug("Invalid YourPlace version number")
		return false, 0, "", nil
	}
	actionCode := matches[2]
	if !security.RegexMatch(`^[a-z]{1,5}$`, actionCode) {
		core.LogDebug("Invalid YourPlace action code")
		return false, 0, "", nil
	}
	var payloadObject map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(matches[3]))
	err = decoder.Decode(&payloadObject)
	if err != nil {
		core.LogDebug("Could not unmarshal YourPlace transaction payload into an object")
		return false, 0, "", nil
	}
	return true, versionNumber, actionCode, payloadObject
}
func isValidBurnAddress(blockchain string, toAddress string) bool {
	if blockchain == "base" {
		return toAddress == burnAddressETH
	}
	if blockchain == "ethereum" {
		return toAddress == burnAddressETHDead
	}
	return false
}
func validateProfileColors(colorsMap map[string]interface{}) map[string]string {
	allowedColorKeys := []string{
		"danger", "dark", "light", "link", "primary",
		"quaternary", "secondary", "success", "tertiary", "text", "warning",
	}
	validColors := make(map[string]string)
	for _, key := range allowedColorKeys {
		if val, exists := colorsMap[key]; exists {
			if colorStr, ok := val.(string); ok {
				if security.IsValidHexColor(colorStr) {
					validColors[key] = colorStr
				}
			}
		}
	}
	return validColors
}
func tokenizeYourPlaceTransaction(blockchain string, transaction map[string]interface{}, timestamp uint64, blockNumber uint64) {
	// Pattern-based tokenization and database storage of YourPlace transactions
	data := transaction["input"].(string)[2:]
	decodedDataBytes, err := hex.DecodeString(data)
	if err != nil {
		core.LogDebug("Could not decode YourPlace transaction: " + err.Error())
		return
	}
	decodedDataStr := string(decodedDataBytes)
	isValid, versionNumber, actionCode, payloadObject := isValidYourPlacePayload(decodedDataStr)
	if !isValid {
		core.LogDebug("Could not decode YourPlace transaction: ")
		return
	}

	txHash := transaction["hash"].(string)
	fromAddress := transaction["from"].(string)
	toAddress := transaction["to"].(string)
	parentTxHash := ""
	amountHexStr := transaction["value"].(string)[2:]
	amountInt, _ := strconv.ParseUint(amountHexStr, 16, 64)
	actionPrefix := actionCode[0]
	actionPostfix := actionCode[1:]

	if versionNumber == 1 {
		switch actionPrefix {
		case 'p': // Post Actions
			if !isValidBurnAddress(blockchain, toAddress) {
				core.LogDebug("Post action not sent to burn address")
				return
			}
			switch actionPostfix {
			case "":
				if !handlePostTransaction(payloadObject, txHash, blockchain, fromAddress, parentTxHash, amountInt, timestamp, blockNumber) {
					break
				}
			case "a":
				if !handlePostTransactionAttachment(payloadObject, txHash, blockchain, fromAddress, parentTxHash, amountInt, timestamp, blockNumber) {
					break
				}
			}
			break
		case 'c': // Comment Actions
			switch actionPostfix {
			case "": // Plain comment
				if !handleCommentTransaction(payloadObject, txHash, blockchain, fromAddress, amountInt, timestamp, blockNumber) {
					break
				}
			case "a": // Comment with attachments
				if !handleCommentTransactionAttachment(payloadObject, txHash, blockchain, fromAddress, amountInt, timestamp, blockNumber) {
					break
				}
			}
			break
		case 'r': // Reaction Actions
			switch actionPostfix {
			case "l": // Like
				if !handleLikeTransaction(payloadObject, txHash, blockchain, fromAddress, timestamp) {
					break
				}
			case "dl": // Dislike
				if !handleDislikeTransaction(payloadObject, txHash, blockchain, fromAddress, timestamp) {
					break
				}
			case "e": // Emoji reaction
				if !handleEmojiReactionTransaction(payloadObject, txHash, blockchain, fromAddress, timestamp) {
					break
				}
			}
			break
		case 'f': // Follow Actions
			switch actionPostfix {
			case "": // Follow user (directed to recipient)
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
			case "u": // Unfollow user (directed to recipient)
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
			case "h": // Follow hashtag (to burn address)
				if !isValidBurnAddress(blockchain, toAddress) {
					return
				}
				// TODO: Implement hashtag follow storage
				break
			case "uh": // Unfollow hashtag (to burn address)
				if !isValidBurnAddress(blockchain, toAddress) {
					return
				}
				// TODO: Implement hashtag unfollow storage
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
			case "c":
				colorsRaw, ok1 := payloadObject["c"]
				if !ok1 {
					core.LogDebug("Metadata action missing required colors field")
					break
				}
				colorsMap, ok2 := colorsRaw.(map[string]interface{})
				if !ok2 {
					core.LogDebug("Metadata action colors field is not an object")
					break
				}
				validColors := validateProfileColors(colorsMap)
				if len(validColors) > 0 {
					colorsJSON, err := json.Marshal(validColors)
					if err != nil {
						break
					}
					_Database.OnchainMC(blockchain, fromAddress, string(colorsJSON), timestamp)
				}
				break
			case "bot":
				botRaw, ok1 := payloadObject["bot"]
				if !ok1 {
					core.LogDebug("Metadata action missing required bot field")
					break
				}
				botVal, ok2 := botRaw.(bool)
				if !ok2 {
					core.LogDebug("Metadata action bot field is not a boolean")
					break
				}
				if !botVal {
					core.LogDebug("Metadata action bot flag is a one-way door, ignoring false value")
					break
				}
				_Database.OnchainMBot(blockchain, fromAddress, botVal, timestamp)
				break
			case "nsfw":
				nsfwRaw, ok1 := payloadObject["nsfw"]
				if !ok1 {
					core.LogDebug("Metadata action missing required nsfw field")
					break
				}
				nsfwVal, ok2 := nsfwRaw.(bool)
				if !ok2 {
					core.LogDebug("Metadata action nsfw field is not a boolean")
					break
				}
				if !nsfwVal {
					core.LogDebug("Metadata action nsfw flag is a one-way door, ignoring false value")
					break
				}
				_Database.OnchainMNsfw(blockchain, fromAddress, nsfwVal, timestamp)
				break
			case "v":
				vertical, ok1 := payloadObject["v"]
				if !ok1 {
					core.LogDebug("Metadata action missing required vertical field")
					break
				}
				verticalStr, ok2 := vertical.(string)
				if !ok2 {
					core.LogDebug("Metadata action vertical field is not a string")
					break
				}
				if security.IsValidVertical(verticalStr) {
					_Database.OnchainMV(blockchain, fromAddress, verticalStr, timestamp)
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
		case 's': // Settings Actions (to burn address)
			if !isValidBurnAddress(blockchain, toAddress) {
				return
			}
			// TODO: Implement settings storage
			break
		default:
			core.LogDebug("Unknown YourPlace transaction action: " + txHash)
		}
	}
}

// --- Transaction Parsing Functions --- //
func handlePostTransaction(payloadObject map[string]interface{}, txHash, blockchain, fromAddress, parentTxHash string, amountInt uint64, timestamp uint64, blockNumber uint64) bool {
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
	postTextStr = security.SanitizeNonPrintable(postTextStr)
	_Database.OnchainP(txHash, blockchain, fromAddress, parentTxHash, amountInt, timestamp, postTextStr)
	return true
}
func handlePostTransactionAttachment(payloadObject map[string]interface{}, txHash, blockchain, fromAddress, parentTxHash string, amountInt uint64, timestamp uint64, blockNumber uint64) bool {
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
	_Database.OnchainPA(txHash, blockchain, fromAddress, parentTxHash, amountInt, timestamp, postTextStr, parsedAttachments)
	return true
}

// --- Comment Transaction Parsing Functions --- //
func handleCommentTransaction(payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, amountInt uint64, timestamp uint64, blockNumber uint64) bool {
	targetTxHash, ok1 := payloadObject["t"]
	commentText, ok2 := payloadObject["p"]
	if !ok1 || !ok2 {
		core.LogDebug("Comment Action: missing required fields t or p")
		return false
	}
	targetTxHashStr, ok1 := targetTxHash.(string)
	commentTextStr, ok2 := commentText.(string)
	if !ok1 || !ok2 {
		core.LogDebug("Comment Action: fields are not strings")
		return false
	}
	if !security.IsValidTxHash(targetTxHashStr, blockchain) {
		core.LogDebug("Comment Action: invalid target transaction hash")
		return false
	}
	commentTextStr = security.SanitizeNonPrintable(commentTextStr)
	_Database.OnchainC(txHash, blockchain, fromAddress, targetTxHashStr, amountInt, timestamp, commentTextStr)
	return true
}
func handleCommentTransactionAttachment(payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, amountInt uint64, timestamp uint64, blockNumber uint64) bool {
	targetTxHash, ok1 := payloadObject["t"]
	commentText, ok2 := payloadObject["p"]
	attachmentsRaw, ok3 := payloadObject["a"]
	if !ok1 || !ok2 || !ok3 {
		core.LogDebug("Comment Attach Action: missing required fields")
		return false
	}
	targetTxHashStr, ok1 := targetTxHash.(string)
	commentTextStr, ok2 := commentText.(string)
	attachmentsArray, ok3 := attachmentsRaw.([]interface{})
	if !ok1 || !ok2 || !ok3 {
		core.LogDebug("Comment Attach Action: fields are not properly typed")
		return false
	}
	if !security.IsValidTxHash(targetTxHashStr, blockchain) {
		core.LogDebug("Comment Attach Action: invalid target transaction hash")
		return false
	}
	parsedAttachments := []db.Attachment{}
	for _, attachment := range attachmentsArray {
		attachmentArray, ok := attachment.([]interface{})
		if !ok {
			core.LogDebug("Comment Attach Action: attachment is not array")
			return false
		}
		if len(attachmentArray) != 4 {
			core.LogDebug("Comment Attach Action: attachment array length is not 4")
			return false
		}
		parsedURL, okURL := attachmentArray[0].(string)
		parsedMimeType, okMimeType := attachmentArray[1].(string)
		sizeFloat, okSize := attachmentArray[2].(float64)
		fileName, okFileName := attachmentArray[3].(string)
		if !okURL || !okMimeType || !okSize || !okFileName {
			core.LogDebug("Comment Attach Action: attachment values are not properly typed")
			return false
		}
		if !security.IsValidIndexedFilename(fileName) {
			core.LogDebug("Comment Attach Action: invalid filename")
			return false
		}
		if !security.IsValidURL(parsedURL) && !security.IsValidCID(parsedURL) {
			core.LogDebug("Comment Attach Action: invalid URL or CID")
			return false
		}
		if sizeFloat < 0 {
			core.LogDebug("Comment Attach Action: negative file size")
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
	commentTextStr = security.SanitizeNonPrintable(commentTextStr)
	_Database.OnchainCA(txHash, blockchain, fromAddress, targetTxHashStr, amountInt, timestamp, commentTextStr, parsedAttachments)
	return true
}

// --- Reaction Transaction Parsing Functions --- //
func handleLikeTransaction(payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, timestamp uint64) bool {
	targetTxHash, ok1 := payloadObject["t"]
	if !ok1 {
		core.LogDebug("Like Action: missing target transaction hash")
		return false
	}
	targetTxHashStr, ok := targetTxHash.(string)
	if !ok {
		core.LogDebug("Like Action: target is not a string")
		return false
	}
	if !security.IsValidTxHash(targetTxHashStr, blockchain) {
		core.LogDebug("Like Action: invalid target transaction hash")
		return false
	}
	targetType := "post"
	if targetTypeRaw, ok := payloadObject["y"]; ok {
		if tt, ok := targetTypeRaw.(string); ok && (tt == "post" || tt == "comment") {
			targetType = tt
		}
	}
	_Database.OnchainR(txHash, blockchain, fromAddress, targetTxHashStr, targetType, "like", timestamp)
	return true
}
func handleDislikeTransaction(payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, timestamp uint64) bool {
	targetTxHash, ok1 := payloadObject["t"]
	if !ok1 {
		core.LogDebug("Dislike Action: missing target transaction hash")
		return false
	}
	targetTxHashStr, ok := targetTxHash.(string)
	if !ok {
		core.LogDebug("Dislike Action: target is not a string")
		return false
	}
	if !security.IsValidTxHash(targetTxHashStr, blockchain) {
		core.LogDebug("Dislike Action: invalid target transaction hash")
		return false
	}
	targetType := "post"
	if targetTypeRaw, ok := payloadObject["y"]; ok {
		if tt, ok := targetTypeRaw.(string); ok && (tt == "post" || tt == "comment") {
			targetType = tt
		}
	}
	_Database.OnchainR(txHash, blockchain, fromAddress, targetTxHashStr, targetType, "dislike", timestamp)
	return true
}
func handleEmojiReactionTransaction(payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, timestamp uint64) bool {
	targetTxHash, ok1 := payloadObject["t"]
	emoji, ok2 := payloadObject["e"]
	if !ok1 || !ok2 {
		core.LogDebug("Emoji Reaction Action: missing required fields")
		return false
	}
	targetTxHashStr, ok1 := targetTxHash.(string)
	emojiStr, ok2 := emoji.(string)
	if !ok1 || !ok2 {
		core.LogDebug("Emoji Reaction Action: fields are not strings")
		return false
	}
	if !security.IsValidTxHash(targetTxHashStr, blockchain) {
		core.LogDebug("Emoji Reaction Action: invalid target transaction hash")
		return false
	}
	if len(emojiStr) == 0 || len(emojiStr) > 32 {
		core.LogDebug("Emoji Reaction Action: invalid emoji length")
		return false
	}
	emojiStr = security.SanitizeNonPrintable(emojiStr)
	targetType := "post"
	if targetTypeRaw, ok := payloadObject["y"]; ok {
		if tt, ok := targetTypeRaw.(string); ok && (tt == "post" || tt == "comment") {
			targetType = tt
		}
	}
	_Database.OnchainR(txHash, blockchain, fromAddress, targetTxHashStr, targetType, emojiStr, timestamp)
	return true
}
