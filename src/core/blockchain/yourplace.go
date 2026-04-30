package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/security"
	"YourPlace/src/core/services"
	"encoding/hex"
	"encoding/json"
	"net/url"
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

type yourPlaceTargetPolicy struct {
	allowBurnTarget bool
	allowSelfTarget bool
}

type yourPlaceTransactionContext struct {
	database     *db.Database
	blockchain   string
	txHash       string
	fromAddress  string
	toAddress    string
	payload      string
	amount       uint64
	timestamp    uint64
	blockNumber  uint64
	targetPolicy yourPlaceTargetPolicy
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
func isValidYourPlaceTarget(txnContext yourPlaceTransactionContext) bool {
	if txnContext.targetPolicy.allowSelfTarget && txnContext.toAddress == txnContext.fromAddress {
		return true
	}
	if txnContext.targetPolicy.allowBurnTarget && isValidBurnAddress(txnContext.blockchain, txnContext.toAddress) {
		return true
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
	amountHexStr := transaction["value"].(string)[2:]
	amountInt, _ := strconv.ParseUint(amountHexStr, 16, 64)
	txnContext := yourPlaceTransactionContext{
		database:    _Database,
		blockchain:  blockchain,
		txHash:      transaction["hash"].(string),
		fromAddress: transaction["from"].(string),
		toAddress:   transaction["to"].(string),
		payload:     string(decodedDataBytes),
		amount:      amountInt,
		timestamp:   timestamp,
		blockNumber: blockNumber,
		targetPolicy: yourPlaceTargetPolicy{
			allowBurnTarget: true,
		},
	}
	tokenizeYourPlacePayload(txnContext)
}
func tokenizeYourPlacePayload(txnContext yourPlaceTransactionContext) {
	isValid, versionNumber, actionCode, payloadObject := isValidYourPlacePayload(txnContext.payload)
	if !isValid {
		core.LogDebug("Could not decode YourPlace transaction: " + txnContext.txHash)
		return
	}
	parentTxHash := ""
	actionPrefix := actionCode[0]
	actionPostfix := actionCode[1:]

	if versionNumber == 1 {
		switch actionPrefix {
		case 'p': // Post Actions
			if !isValidYourPlaceTarget(txnContext) {
				core.LogDebug("Post action sent to invalid target")
				return
			}
			switch actionPostfix {
			case "":
				if !handlePostTransaction(txnContext.database, payloadObject, txnContext.txHash, txnContext.blockchain, txnContext.fromAddress, parentTxHash, txnContext.amount, txnContext.timestamp, txnContext.blockNumber) {
					break
				}
			case "a":
				if !handlePostTransactionAttachment(txnContext.database, payloadObject, txnContext.txHash, txnContext.blockchain, txnContext.fromAddress, parentTxHash, txnContext.amount, txnContext.timestamp, txnContext.blockNumber) {
					break
				}
			case "f":
				if !handleFileTransactionPublish(txnContext.database, payloadObject, txnContext.txHash, txnContext.blockchain, txnContext.fromAddress, txnContext.timestamp) {
					break
				}
			case "fd":
				if !handleFileTransactionDelete(txnContext.database, payloadObject, txnContext.txHash, txnContext.blockchain, txnContext.fromAddress, txnContext.timestamp) {
					break
				}
			}
			break
		case 'c': // Comment Actions
			switch actionPostfix {
			case "":
				if !handleCommentTransaction(txnContext.database, payloadObject, txnContext.txHash, txnContext.blockchain, txnContext.fromAddress, txnContext.amount, txnContext.timestamp, txnContext.blockNumber) {
					break
				}
			case "a":
				if !handleCommentTransactionAttachment(txnContext.database, payloadObject, txnContext.txHash, txnContext.blockchain, txnContext.fromAddress, txnContext.amount, txnContext.timestamp, txnContext.blockNumber) {
					break
				}
			}
			break
		case 'r': // Reaction Actions
			switch actionPostfix {
			case "l":
				if !handleLikeTransaction(txnContext.database, payloadObject, txnContext.txHash, txnContext.blockchain, txnContext.fromAddress, txnContext.timestamp) {
					break
				}
			case "dl":
				if !handleDislikeTransaction(txnContext.database, payloadObject, txnContext.txHash, txnContext.blockchain, txnContext.fromAddress, txnContext.timestamp) {
					break
				}
			case "e":
				if !handleEmojiReactionTransaction(txnContext.database, payloadObject, txnContext.txHash, txnContext.blockchain, txnContext.fromAddress, txnContext.timestamp) {
					break
				}
			}
			break
		case 'f': // Follow Actions
			switch actionPostfix {
			case "":
				if !handleFollowTransaction(txnContext.database, payloadObject, txnContext.txHash, txnContext.blockchain, txnContext.fromAddress, txnContext.timestamp) {
					break
				}
			case "u":
				if !handleUnfollowTransaction(txnContext.database, payloadObject, txnContext.txHash, txnContext.blockchain, txnContext.fromAddress, txnContext.timestamp) {
					break
				}
			case "h":
				if !isValidYourPlaceTarget(txnContext) {
					return
				}
				// TODO: Implement hashtag follow storage
				break
			case "uh":
				if !isValidYourPlaceTarget(txnContext) {
					return
				}
				// TODO: Implement hashtag unfollow storage
				break
			}
			break
		case 'm': // Metadata Actions
			if !isValidYourPlaceTarget(txnContext) {
				return
			}
			handleMetadataTransaction(txnContext.database, payloadObject, txnContext.blockchain, txnContext.fromAddress, actionPostfix, txnContext.timestamp)
			break
		case 'b': // Blocking Actions
		case 's': // Settings Actions
			if !isValidYourPlaceTarget(txnContext) {
				return
			}
			// TODO: Implement settings storage
			break
		default:
			core.LogDebug("Unknown YourPlace transaction action: " + txnContext.txHash)
		}
	}
}

// --- Transaction Parsing Functions --- //
func handleFollowTransaction(database *db.Database, payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, timestamp uint64) bool {
	blockchainPayload, ok1 := payloadObject["b"]
	addressPayload, ok2 := payloadObject["a"]
	if !ok1 || !ok2 {
		core.LogDebug("Follow action missing required fields")
		return false
	}
	blockchainStr, ok1 := blockchainPayload.(string)
	addressStr, ok2 := addressPayload.(string)
	if !ok1 || !ok2 {
		core.LogDebug("Follow action fields are not strings")
		return false
	}
	if !security.IsValidBlockchain(blockchainStr) {
		core.LogDebug("Invalid blockchain in follow action")
		return false
	}
	if !security.IsValidAddress(addressStr, blockchainStr) {
		core.LogDebug("Invalid address in follow action")
		return false
	}
	if fromAddress == addressStr && blockchain == blockchainStr {
		return false
	}
	database.OnchainF(txHash, blockchain, fromAddress, blockchain, addressStr, blockchainStr, timestamp)
	return true
}
func handleUnfollowTransaction(database *db.Database, payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, timestamp uint64) bool {
	blockchainPayload, ok1 := payloadObject["b"]
	addressPayload, ok2 := payloadObject["a"]
	if !ok1 || !ok2 {
		core.LogDebug("Unfollow action missing required fields")
		return false
	}
	blockchainStr, ok1 := blockchainPayload.(string)
	addressStr, ok2 := addressPayload.(string)
	if !ok1 || !ok2 {
		core.LogDebug("Unfollow action fields are not strings")
		return false
	}
	if !security.IsValidBlockchain(blockchainStr) {
		core.LogDebug("Invalid blockchain in unfollow action")
		return false
	}
	if !security.IsValidAddress(addressStr, blockchainStr) {
		core.LogDebug("Invalid address in unfollow action")
		return false
	}
	if fromAddress == addressStr && blockchain == blockchainStr {
		return false
	}
	database.OnchainFU(txHash, blockchain, fromAddress, blockchain, addressStr, blockchainStr, timestamp)
	return true
}
func handleMetadataTransaction(database *db.Database, payloadObject map[string]interface{}, blockchain, fromAddress, actionPostfix string, timestamp uint64) {
	switch actionPostfix {
	case "n":
		name, ok1 := payloadObject["n"]
		if !ok1 {
			core.LogDebug("Metadata action missing required name field")
			return
		}
		nameStr, ok2 := name.(string)
		if !ok2 {
			core.LogDebug("Metadata action name field is not a string")
			return
		}
		nameStr = security.SanitizeNonPrintable(nameStr)
		database.OnchainMN(blockchain, fromAddress, nameStr, timestamp)
	case "a":
		avatar, ok1 := payloadObject["a"]
		if !ok1 {
			core.LogDebug("Metadata action missing required avatar field")
			return
		}
		avatarStr, ok2 := avatar.(string)
		if !ok2 {
			core.LogDebug("Metadata action avatar field is not a string")
			return
		}
		avatarStr = security.SanitizeNonPrintable(avatarStr)
		if security.IsValidURL(avatarStr) || security.IsValidCID(avatarStr) {
			database.OnchainMA(blockchain, fromAddress, avatarStr, timestamp)
		}
	case "b":
		banner, ok1 := payloadObject["b"]
		if !ok1 {
			core.LogDebug("Metadata action missing required banner field")
			return
		}
		bannerStr, ok2 := banner.(string)
		if !ok2 {
			core.LogDebug("Metadata action banner field is not a string")
			return
		}
		bannerStr = security.SanitizeNonPrintable(bannerStr)
		if security.IsValidURL(bannerStr) || security.IsValidCID(bannerStr) {
			database.OnchainMB(blockchain, fromAddress, bannerStr, timestamp)
		}
	case "c":
		colorsRaw, ok1 := payloadObject["c"]
		if !ok1 {
			core.LogDebug("Metadata action missing required colors field")
			return
		}
		colorsMap, ok2 := colorsRaw.(map[string]interface{})
		if !ok2 {
			core.LogDebug("Metadata action colors field is not an object")
			return
		}
		validColors := validateProfileColors(colorsMap)
		if len(validColors) > 0 {
			colorsJSON, err := json.Marshal(validColors)
			if err != nil {
				return
			}
			database.OnchainMC(blockchain, fromAddress, string(colorsJSON), timestamp)
		}
	case "bot":
		botRaw, ok1 := payloadObject["bot"]
		if !ok1 {
			core.LogDebug("Metadata action missing required bot field")
			return
		}
		botVal, ok2 := botRaw.(bool)
		if !ok2 {
			core.LogDebug("Metadata action bot field is not a boolean")
			return
		}
		if !botVal {
			core.LogDebug("Metadata action bot flag is a one-way door, ignoring false value")
			return
		}
		database.OnchainMBot(blockchain, fromAddress, botVal, timestamp)
	case "nsfw":
		nsfwRaw, ok1 := payloadObject["nsfw"]
		if !ok1 {
			core.LogDebug("Metadata action missing required nsfw field")
			return
		}
		nsfwVal, ok2 := nsfwRaw.(bool)
		if !ok2 {
			core.LogDebug("Metadata action nsfw field is not a boolean")
			return
		}
		if !nsfwVal {
			core.LogDebug("Metadata action nsfw flag is a one-way door, ignoring false value")
			return
		}
		database.OnchainMNsfw(blockchain, fromAddress, nsfwVal, timestamp)
	case "v":
		vertical, ok1 := payloadObject["v"]
		if !ok1 {
			core.LogDebug("Metadata action missing required vertical field")
			return
		}
		verticalStr, ok2 := vertical.(string)
		if !ok2 {
			core.LogDebug("Metadata action vertical field is not a string")
			return
		}
		if security.IsValidVertical(verticalStr) {
			database.OnchainMV(blockchain, fromAddress, verticalStr, timestamp)
		}
	case "l":
		location, ok1 := payloadObject["l"]
		if !ok1 {
			core.LogDebug("Metadata action missing required location field")
			return
		}
		locationStr, ok2 := location.(string)
		if !ok2 {
			core.LogDebug("Metadata action location field is not a string")
			return
		}
		locationStr = security.SanitizeNonPrintable(locationStr)
		database.OnchainML(blockchain, fromAddress, locationStr, timestamp)
	case "m":
		music, ok1 := payloadObject["m"]
		if !ok1 {
			core.LogDebug("Metadata action missing required music field")
			return
		}
		musicStr, ok2 := music.(string)
		if !ok2 {
			core.LogDebug("Metadata action music field is not a string")
			return
		}
		musicStr = security.SanitizeNonPrintable(musicStr)
		if musicStr == "" {
			database.OnchainMM(blockchain, fromAddress, "", timestamp)
			return
		}
		if valid, _ := services.IsValidSpotifyUri(musicStr); valid {
			database.OnchainMM(blockchain, fromAddress, musicStr, timestamp)
		} else {
			core.LogDebug("Metadata music action URL is not a recognized provider")
		}
	case "w":
		website, ok1 := payloadObject["w"]
		if !ok1 {
			core.LogDebug("Metadata action missing required website field")
			return
		}
		websiteStr, ok2 := website.(string)
		if !ok2 {
			core.LogDebug("Metadata action website field is not a string")
			return
		}
		websiteStr = security.SanitizeNonPrintable(websiteStr)
		if security.IsValidURL(websiteStr) && len(websiteStr) > 0 {
			database.OnchainMW(blockchain, fromAddress, websiteStr, timestamp)
		}
	case "d":
		description, ok1 := payloadObject["d"]
		if !ok1 {
			core.LogDebug("Metadata action missing required description field")
			return
		}
		descriptionStr, ok2 := description.(string)
		if !ok2 {
			core.LogDebug("Metadata action description field is not a string")
			return
		}
		descriptionStr = security.SanitizeNonPrintable(descriptionStr)
		if len(descriptionStr) > 0 {
			database.OnchainMD(blockchain, fromAddress, descriptionStr, timestamp)
		}
	}
}
func handlePostTransaction(database *db.Database, payloadObject map[string]interface{}, txHash, blockchain, fromAddress, parentTxHash string, amountInt uint64, timestamp uint64, blockNumber uint64) bool {
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
	database.OnchainP(txHash, blockchain, fromAddress, parentTxHash, amountInt, timestamp, postTextStr)
	return true
}
func normalizeAttachmentCID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "ipfs://") {
		value = strings.TrimPrefix(value, "ipfs://")
	}
	if idx := strings.IndexAny(value, "/?#"); idx != -1 {
		value = value[:idx]
	}
	if dot := strings.Index(value, "."); dot != -1 {
		candidate := value[:dot]
		if security.IsValidCID(candidate) {
			return candidate
		}
	}
	if security.IsValidCID(value) {
		return strings.TrimPrefix(value, "ipfs://")
	}
	parsedURL, err := url.Parse(value)
	if err != nil {
		return ""
	}
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	for index, part := range pathParts {
		if part == "ipfs" && index+1 < len(pathParts) && security.IsValidCID(pathParts[index+1]) {
			return pathParts[index+1]
		}
	}
	hostParts := strings.Split(parsedURL.Hostname(), ".")
	if len(hostParts) > 0 && security.IsValidCID(hostParts[0]) {
		return hostParts[0]
	}
	return ""
}
func parseYourPlaceAttachments(attachmentsRaw interface{}) ([]db.Attachment, bool) {
	attachmentsArray, ok := attachmentsRaw.([]interface{})
	if !ok {
		core.LogDebug("Attachment payload is not properly typed")
		return nil, false
	}
	parsedAttachments := []db.Attachment{}
	for _, attachment := range attachmentsArray {
		attachmentArray, ok := attachment.([]interface{})
		if !ok {
			core.LogDebug("Attachment payload entry is not an array")
			return nil, false
		}
		if len(attachmentArray) != 4 {
			core.LogDebug("Attachment payload entry length is not 4")
			return nil, false
		}
		cidValue, okCID := attachmentArray[0].(string)
		parsedMimeType, okMimeType := attachmentArray[1].(string)
		sizeFloat, okSize := attachmentArray[2].(float64)
		fileName, okFileName := attachmentArray[3].(string)
		if !okCID || !okMimeType || !okSize || !okFileName {
			core.LogDebug("Attachment payload entry values are not properly typed")
			return nil, false
		}
		if !security.IsValidIndexedFilename(fileName) {
			core.LogDebug("Attachment payload entry contains an invalid filename")
			return nil, false
		}
		cid := normalizeAttachmentCID(cidValue)
		if cid == "" {
			core.LogDebug("Attachment payload entry does not contain a valid CID")
			return nil, false
		}
		if sizeFloat < 0 {
			core.LogDebug("Attachment payload entry contains negative file size")
			return nil, false
		}
		parsedAttachments = append(parsedAttachments, db.Attachment{
			CID:      cid,
			MimeType: parsedMimeType,
			FileSize: uint64(sizeFloat),
			FileName: fileName,
		})
	}
	return parsedAttachments, true
}
func parseYourPlaceCIDList(cidsRaw interface{}) ([]string, bool) {
	cidArray, ok := cidsRaw.([]interface{})
	if !ok {
		core.LogDebug("CID payload is not properly typed")
		return nil, false
	}
	if len(cidArray) == 0 {
		core.LogDebug("CID payload is empty")
		return nil, false
	}
	parsedCIDs := []string{}
	seenCIDs := make(map[string]struct{})
	for _, cidEntry := range cidArray {
		cidValue, ok := cidEntry.(string)
		if !ok {
			core.LogDebug("CID payload entry is not a string")
			return nil, false
		}
		cid := normalizeAttachmentCID(cidValue)
		if cid == "" {
			core.LogDebug("CID payload entry is invalid")
			return nil, false
		}
		if _, exists := seenCIDs[cid]; exists {
			continue
		}
		seenCIDs[cid] = struct{}{}
		parsedCIDs = append(parsedCIDs, cid)
	}
	if len(parsedCIDs) == 0 {
		core.LogDebug("CID payload did not contain any valid entries")
		return nil, false
	}
	return parsedCIDs, true
}
func handlePostTransactionAttachment(database *db.Database, payloadObject map[string]interface{}, txHash, blockchain, fromAddress, parentTxHash string, amountInt uint64, timestamp uint64, blockNumber uint64) bool {
	postText, ok1 := payloadObject["p"]
	attachmentsRaw, ok2 := payloadObject["a"]
	if !ok1 || !ok2 {
		core.LogDebug("Post attach action missing required fields")
		return false
	}
	postTextStr, ok3 := postText.(string)
	if !ok3 {
		core.LogDebug("Post attach action fields are not properly typed")
		return false
	}
	parsedAttachments, ok := parseYourPlaceAttachments(attachmentsRaw)
	if !ok {
		return false
	}
	postTextStr = security.SanitizeNonPrintable(postTextStr)
	database.OnchainPA(txHash, blockchain, fromAddress, parentTxHash, amountInt, timestamp, postTextStr, parsedAttachments)
	return true
}
func handleFileTransactionPublish(database *db.Database, payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, timestamp uint64) bool {
	attachmentsRaw, ok := payloadObject["a"]
	if !ok {
		core.LogDebug("File publish action missing required attachments field")
		return false
	}
	parsedAttachments, ok := parseYourPlaceAttachments(attachmentsRaw)
	if !ok {
		return false
	}
	database.OnchainPF(txHash, blockchain, fromAddress, timestamp, parsedAttachments)
	return true
}
func handleFileTransactionDelete(database *db.Database, payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, timestamp uint64) bool {
	cidsRaw, ok := payloadObject["c"]
	if !ok {
		core.LogDebug("File delete action missing required CID field")
		return false
	}
	parsedCIDs, ok := parseYourPlaceCIDList(cidsRaw)
	if !ok {
		return false
	}
	database.OnchainPFD(txHash, blockchain, fromAddress, timestamp, parsedCIDs)
	return true
}

// --- Comment Transaction Parsing Functions --- //
func handleCommentTransaction(database *db.Database, payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, amountInt uint64, timestamp uint64, blockNumber uint64) bool {
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
	database.OnchainC(txHash, blockchain, fromAddress, targetTxHashStr, amountInt, timestamp, commentTextStr)
	return true
}
func handleCommentTransactionAttachment(database *db.Database, payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, amountInt uint64, timestamp uint64, blockNumber uint64) bool {
	targetTxHash, ok1 := payloadObject["t"]
	commentText, ok2 := payloadObject["p"]
	attachmentsRaw, ok3 := payloadObject["a"]
	if !ok1 || !ok2 || !ok3 {
		core.LogDebug("Comment Attach Action: missing required fields")
		return false
	}
	targetTxHashStr, ok1 := targetTxHash.(string)
	commentTextStr, ok2 := commentText.(string)
	if !ok1 || !ok2 {
		core.LogDebug("Comment Attach Action: fields are not properly typed")
		return false
	}
	if !security.IsValidTxHash(targetTxHashStr, blockchain) {
		core.LogDebug("Comment Attach Action: invalid target transaction hash")
		return false
	}
	parsedAttachments, ok := parseYourPlaceAttachments(attachmentsRaw)
	if !ok {
		return false
	}
	commentTextStr = security.SanitizeNonPrintable(commentTextStr)
	database.OnchainCA(txHash, blockchain, fromAddress, targetTxHashStr, amountInt, timestamp, commentTextStr, parsedAttachments)
	return true
}

// --- Reaction Transaction Parsing Functions --- //
func handleLikeTransaction(database *db.Database, payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, timestamp uint64) bool {
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
	database.OnchainR(txHash, blockchain, fromAddress, targetTxHashStr, targetType, "like", timestamp)
	return true
}
func handleDislikeTransaction(database *db.Database, payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, timestamp uint64) bool {
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
	database.OnchainR(txHash, blockchain, fromAddress, targetTxHashStr, targetType, "dislike", timestamp)
	return true
}
func handleEmojiReactionTransaction(database *db.Database, payloadObject map[string]interface{}, txHash, blockchain, fromAddress string, timestamp uint64) bool {
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
	database.OnchainR(txHash, blockchain, fromAddress, targetTxHashStr, targetType, emojiStr, timestamp)
	return true
}
