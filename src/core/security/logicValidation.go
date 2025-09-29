package security

import (
	"YourPlace/src/core"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	_algotypes "github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
)

func CheckPasswordComplexity(password string) (bool, error) {
	if len(password) < 10 {
		return false, errors.New("password too short")
	}
	re1 := regexp.MustCompile(`\d+`)
	match1 := re1.FindString(password)
	if match1 == "" {
		return false, errors.New("password must contain at least one number")
	}
	re2 := regexp.MustCompile(`[a-z]`)
	match2 := re2.FindString(password)
	if match2 == "" {
		return false, errors.New("password must contain at least one lowercase letter")
	}
	re3 := regexp.MustCompile(`[A-Z]`)
	match3 := re3.FindString(password)
	if match3 == "" {
		return false, errors.New("password must contain at least one uppercase letter")
	} else {
		return true, nil
	}
}
func GetFileType(path string) (string, string) {
	// Checks if the file is actually the type it claims to be
	// Returns (true, "filetype") if the file is the type it claims to be
	// https://en.wikipedia.org/wiki/List_of_file_signatures & https://www.garykessler.net/library/file_sigs.html
	type FileType string
	const (
		TypeUnknown FileType = "unknown"
		TypeAVI     FileType = "avi"
		TypeJPEG    FileType = "jpeg"
		TypePNG     FileType = "png"
		TypeGIF     FileType = "gif"
		TypeMP4     FileType = "mp4"
		TypeMKV     FileType = "mkv"
		TypeMOV     FileType = "mov"
		TypeMZ      FileType = "mz"
		TypeELF     FileType = "elf"
		TypePDF     FileType = "pdf"
		TypeMP3     FileType = "mp3"
		TypeWEBP    FileType = "webp"
	)
	var mimeTypes = map[FileType]string{
		TypeUnknown: "application/octet-stream",
		TypeAVI:     "video/x-msvideo",
		TypeJPEG:    "image/jpeg",
		TypePNG:     "image/png",
		TypeGIF:     "image/gif",
		TypeMP4:     "video/mp4",
		TypeMKV:     "video/x-matroska",
		TypeMOV:     "video/quicktime",
		TypeMZ:      "application/x-msdownload",
		TypeELF:     "application/x-executable",
		TypePDF:     "application/pdf",
		TypeMP3:     "audio/mpeg",
		TypeWEBP:    "image/webp",
	}
	var fileSignatures = map[FileType][][]byte{
		TypeAVI:  {{0x52, 0x49, 0x46, 0x46}},
		TypeJPEG: {{0xFF, 0xD8, 0xFF}},
		TypePNG:  {{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}},
		TypeGIF:  {{0x47, 0x49, 0x46, 0x38}},
		TypeMP4:  {{0x25, 0x50, 0x44, 0x46}},
		TypeMKV:  {{0x1A, 0x45, 0xDF, 0xA3}},
		TypeMOV:  {{0x66, 0x74, 0x79, 0x70, 0x71, 0x74, 0x20, 0x20}, {0x6D, 0x6F, 0x6F, 0x76}, {0x66, 0x72, 0x65, 0x65}, {0x6D, 0x64, 0x61, 0x74}, {0x77, 0x69, 0x64, 0x65}, {0x70, 0x6E, 0x6F, 0x74}, {0x73, 0x6B, 0x69, 0x70}},
		TypeMZ:   {{0x4D, 0x5A}},
		TypeELF:  {{0x7F, 0x45, 0x4C, 0x46}},
		TypePDF:  {{0x25, 0x50, 0x44, 0x46}},
		TypeMP3:  {{0x49, 0x44, 0x33}},
	}
	data, err := os.ReadFile(path)
	if err != nil {
		core.LogError("Failed to read file: " + err.Error())
		return string(TypeUnknown), mimeTypes[TypeUnknown]
	}
	detectedType := TypeUnknown
	extUpper := filepath.Ext(path)
	ext := strings.ToLower(extUpper)
	switch ext {
	case ".avi":
		detectedType = TypeAVI
	case ".jpg", ".jpeg":
		detectedType = TypeJPEG
	case ".png":
		detectedType = TypePNG
	case ".gif":
		detectedType = TypeGIF
	case ".mp4":
		detectedType = TypeMP4
	case ".mkv", ".webm":
		detectedType = TypeMKV
	case ".mov":
		detectedType = TypeMOV
	case ".exe", ".dll":
		detectedType = TypeMZ
	case ".pdf":
		detectedType = TypePDF
	case "":
		detectedType = TypeELF
	case ".mp3":
		detectedType = TypeMP3
	case ".webp":
		detectedType = TypeWEBP
	default:
		detectedType = TypeUnknown
	}
	if ext == ".webp" && len(data) >= 12 {
		if bytes.Equal(data[0:4], []byte{0x52, 0x49, 0x46, 0x46}) &&
			bytes.Equal(data[8:12], []byte{0x57, 0x45, 0x42, 0x50}) {
			return string(TypeWEBP), mimeTypes[TypeWEBP]
		}
	}
	for fileType, signatures := range fileSignatures {
		for _, signature := range signatures {
			if bytes.HasPrefix(data, signature) {
				if fileType == detectedType {
					return string(fileType), mimeTypes[fileType]
				}
			}
		}

	}
	return string(TypeUnknown), mimeTypes[TypeUnknown]
}
func IsInParentDirectory(parent string, child string) bool {
	parent, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	child, err = filepath.Abs(child)
	if err != nil {
		return false
	}
	return strings.HasPrefix(child, parent)
}
func IsIntBetween(start int, end int, payload int) bool {
	if payload > end || payload < start {
		return false
	}
	return true
}
func IsPublicIP(ip string) bool {
	// todo
	return false
}
func IsSystemPort(port int) bool {
	if IsIntBetween(0, 1023, port) {
		return true
	}
	return false
}
func IsValidCryptoHex(payload string) bool {
	if len(payload) != 64 {
		return false
	}
	_, err := hex.DecodeString(payload)
	if err != nil {
		return false
	}
	return true
}
func IsValidURL(payload string) bool {
	//if !strings.HasPrefix(strings.ToLower(payload), "https://") { return false }
	_, err := url.Parse(payload)
	if err != nil {
		return false
	}
	return true
}
func IsValidAlgoNetwork(payload string) bool {
	payload = strings.ToLower(payload)
	validNetworks := []string{"mainnet", "testnet", "betanet", "devnet"}
	for _, network := range validNetworks {
		if strings.EqualFold(network, strings.ToLower(payload)) {
			return true
		}
	}
	return false
}
func IsValidAlgodToken(payload string) bool {
	if LengthMatch(payload, 40) { // purestake
		if RegexMatch("[a-zA-Z0-9]{40}", payload) {
			return true
		}
	} else if LengthMatch(payload, 64) { // bisontrails
		if RegexMatch("[a-zA-Z0-9_-]{64}", payload) {
			return true
		}
	} else if LengthMatch(payload, 0) { // algoexplorer
		return true
	}
	return false
}
func IsValidAddress(payload string, blockchain string) bool {
	if blockchain == "base" || blockchain == "eth" {
		return IsValidEthAddress(payload)
	} else if blockchain == "algo" {
		return IsValidAlgoAddress(payload)
	}
	core.LogError("Invalid blockchain")
	return false
}
func IsValidAlgoAddress(payload string) bool {
	_, err := _algotypes.DecodeAddress(payload)
	if err != nil {
		return false
	}
	return true
}
func IsValidAlgoTransaction(payload string) bool {
	if !LengthRange(payload, 10, 2048) {
		fmt.Println("failed transaction length check")
		return false
	}
	return RegexMatch("[a-zA-Z\\d+/]+={0,2}", payload)
}
func IsValidBlockchain(payload string) bool {
	validBlockchains := []string{"algo", "base", "eth"}
	for _, validBlockchain := range validBlockchains {
		if strings.EqualFold(validBlockchain, strings.ToLower(payload)) {
			return true
		}
	}
	return false
}
func IsValidNetwork(payload string) bool {
	if IsValidBlockchain(payload) {
		return true
	}
	otherValidNetworks := []string{"nostr", "farcaster"}
	for _, validNetwork := range otherValidNetworks {
		if strings.EqualFold(validNetwork, strings.ToLower(payload)) {
			return true
		}
	}
	return false
}
func IsValidCID(_cid string) bool {
	_cid = strings.TrimPrefix(_cid, "ipfs://")
	_, err := cid.Decode(_cid)
	if err != nil {
		return false
	}
	return true
}
func IsValidUUID(payload string) bool {
	_, err := uuid.Parse(payload)
	if err != nil {
		return false
	}
	return true
}
func IsValidEthAddress(payload string) bool {
	if !strings.HasPrefix(payload, "0x") {
		return false
	}
	if len(payload) != 42 { // 42 characters = 2 for "0x" + 40 for address
		return false
	}
	if RegexMatch("^0x[0-9a-fA-F]{40}$", payload) { // Check for valid hex characters
		return true
	}
	return false
}
func IsValidENSName(payload string) (bool, string) {
	validSuffixes := map[string]string{
		".base.eth": "base",
		".eth":      "eth",
	}
	hasValidSuffix := false
	var blockchain string
	var namePart string
	for suffix, chain := range validSuffixes {
		if strings.HasSuffix(payload, suffix) {
			hasValidSuffix = true
			blockchain = chain
			namePart = strings.TrimSuffix(payload, suffix)
			break
		}
	}
	if !hasValidSuffix { // must have valid suffix
		return false, ""
	}
	if len(namePart) < 3 { // minimum length
		return false, ""
	}
	if strings.HasPrefix(namePart, "-") || strings.HasSuffix(namePart, "-") { // cannot start or end with hyphen
		return false, ""
	}
	for _, char := range namePart { // check for valid character set
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
			return false, ""
		}
	}
	return true, blockchain
}
func IsValidERC6492Signature(payload string, signature string, address string) bool {
	signatureBytes := []byte(signature)
	var ERC6492_DETECTION_SUFFIX = "6492649264926492649264926492649264926492649264926492649264926492"
	//var ERC6492_DETECTION_HASH = common.HexToHash("0x6492649264926492649264926492649264926492649264926492649264926492")
	if len(signatureBytes) < 32 {
		return false
	}
	suffix := hex.EncodeToString(signatureBytes[len(signatureBytes)-32:])
	if !strings.EqualFold(suffix, ERC6492_DETECTION_SUFFIX) {
		return false
	}
	return false // todo
}
func IsValidEthSignature(payload string, signature string, address string) bool {
	decodedSignature, err := hex.DecodeString(signature[2:])
	if err != nil {
		core.LogError("Failed to decode signature: " + err.Error())
		return false
	}
	if len(decodedSignature) != 65 {
		core.LogError("Invalid signature length")
		return false
	}
	r := decodedSignature[:32]
	s := decodedSignature[32:64]
	if decodedSignature[64] >= 27 {
		decodedSignature[64] -= 27
	}
	if !crypto.ValidateSignatureValues(decodedSignature[64], new(big.Int).SetBytes(r), new(big.Int).SetBytes(s), true) {
		core.LogError("Invalid signature values")
		return false
	}
	decodedPayload, err := hex.DecodeString(payload[2:])
	if err != nil {
		core.LogError("Failed to decode payload: " + err.Error())
		return false
	}
	validateMsg := "\x19Ethereum Signed Message:\n" + strconv.Itoa(len(decodedPayload)) + string(decodedPayload)
	hash := crypto.Keccak256([]byte(validateMsg))
	pubKeyBytes, err := crypto.Ecrecover(hash, decodedSignature)
	if err != nil {
		core.LogError("Failed to recover public key: " + err.Error())
		return false
	}
	pubKey, err := crypto.UnmarshalPubkey(pubKeyBytes)
	if err != nil {
		core.LogError("Failed to unmarshal public key: " + err.Error())
		return false
	}
	ethAddress := crypto.PubkeyToAddress(*pubKey).Hex()
	if strings.EqualFold(address, ethAddress) {
		return true
	}
	core.LogWarn("Signature is invalid")
	return false
}
func IsValidHex(payload string) bool {
	_, err := hex.DecodeString(payload)
	if err != nil {
		return false
	}
	return true
}
func IsValidBase64(payload string) bool {
	if len(payload) == 0 {
		return false
	}
	decoded := Base64DecodeBytes(payload)
	if decoded == nil {
		return false
	}
	return true
}
func IsValidIndexedFilename(fileName string) bool { // Returns bool because we probably shouldn't index a file that actually needs this sanitization.
	if SanitizePathTraversal(fileName) != fileName {
		return false
	}
	if SanitizeCommandInjection(fileName) != fileName {
		return false
	}
	if SanitizeNonPrintable(fileName) != fileName {
		return false
	}
	if !LengthRange(fileName, 1, 255) {
		return false
	}
	return true
}
func IsValidNFDomain(payload string) bool {
	return RegexMatch("^[a-zA-Z0-9]{1,27}\\.algo$", payload)
}
func IsValidPort(port int) bool {
	if IsIntBetween(0, 65535, port) {
		return true
	}
	return false
}
func IsValidYourPlaceVersion(payload string) bool {
	pattern := `^[0-9]+\.[0-9]+\.[0-9]+$`
	matched, err := regexp.MatchString(pattern, payload)
	if err != nil {
		return false
	}
	return matched
}
func IsVersionGreater(current string, latest string) bool {
	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")
	if IsValidYourPlaceVersion(current) == false || IsValidYourPlaceVersion(latest) == false {
		return false
	}
	for i := 0; i < 3; i++ { // Compare each part
		current2, _ := strconv.Atoi(currentParts[i])
		latest2, _ := strconv.Atoi(latestParts[i])
		if latest2 > current2 {
			return true
		}
		if latest2 < current2 {
			return false
		}
	}
	return false // Versions are equal
}
func IsValidWallet(payload string) bool {
	validWallets := []string{"pera", "cbwalletbase"}
	for _, wallet := range validWallets {
		if payload == wallet {
			return true
		}
	}
	return false
}
func IsValidNumberRange(payload int, min int, max int) bool {
	if payload > max || payload < min {
		return false
	}
	return true
}
func IsValidBirthDate(payload int64) bool {
	now := time.Now().Unix()
	thirteenYears := int64(13 * 365.25 * 24 * 60 * 60)
	twoHundredYears := int64(200 * 365.25 * 24 * 60 * 60)
	diff := now - int64(payload)
	return diff >= thirteenYears && diff <= twoHundredYears
}
func IsVideo(filename string) bool {
	// mov files are currently excluded from this check
	fileType := ""
	if filepath.Ext(filename) == ".mov" {
		fileType = "mov"
	} else {
		fileType, _ = GetFileType(filename)
	}
	switch fileType {
	case "avi":
		return true
	case "mkv":
		return true
	case "mov":
		return true
	case "mp4":
		return true
	default:
		return false
	}
}
func IsAllZeros(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}
func IsHttpProtocol(url string) bool {
	lowerURL := strings.ToLower(url)
	if strings.HasPrefix(lowerURL, "https:") {
		return true
	} else if strings.HasPrefix(lowerURL, "http:") {
		return true
	}
	return false
}
func IsHttpsProtocol(url string) bool {
	lowerURL := strings.ToLower(url)
	if strings.HasPrefix(lowerURL, "https:") {
		return true
	}
	return false
}
func IsNewerVersion(current, latest string) bool {
	current = strings.TrimPrefix(current, "v") // drop any 'v' prefix if present
	latest = strings.TrimPrefix(latest, "v")
	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")
	if len(currentParts) != 3 || len(latestParts) != 3 {
		return false // Invalid version format
	}
	for i := 0; i < 3; i++ { // Compare each part, validating they are 1-3 digits
		if !regexp.MustCompile(`^\d{1,3}$`).MatchString(currentParts[i]) ||
			!regexp.MustCompile(`^\d{1,3}$`).MatchString(latestParts[i]) {
			core.LogError(fmt.Sprintf("Invalid version number at position %d - must be 1-3 digits", i))
			return false
		}
		current2, _ := strconv.Atoi(currentParts[i])
		latest2, _ := strconv.Atoi(latestParts[i])
		if latest2 > current2 {
			return true
		}
		if latest2 < current2 {
			return false
		}
	}
	return false // Versions are equal
}
func LengthMatch(payload string, length uint) bool {
	if uint(len(payload)) == length {
		return true
	}
	return false
}
func LengthRange(payload string, minLength uint, maxLength uint) bool {
	if uint(len(payload)) < minLength {
		return false
	} else if uint(len(payload)) > maxLength {
		return false
	}
	return true
}
func RegexMatch(regex string, payload string) bool {
	found, err := regexp.MatchString(regex, payload)
	if err != nil {
		return false
	}
	if found {
		return true
	}
	return false
}
func RecursiveURLDecode(payload string) string {
	decodedStr, err := url.QueryUnescape(payload)
	if err != nil {
		core.LogError("Failed to decode URL: " + err.Error())
		return ""
	}
	if decodedStr == payload {
		return decodedStr
	}
	return RecursiveURLDecode(decodedStr)
}
func SanitizeURL(scheme string, host string, path string) string {
	urlParsed := url.URL{} // prevent URL injection by constructing URL object
	urlParsed.Scheme = scheme
	urlParsed.Host = host
	urlParsed.Path = path
	return urlParsed.String() // URLs String() method uses the EscapedPath() method
}
func SanitizePathTraversal(payload string) string {
	payload = RecursiveURLDecode(payload)
	payload = filepath.Clean(payload)
	payload = strings.ReplaceAll(payload, "..", "")
	payload = strings.ReplaceAll(payload, "./", "")
	payload = strings.ReplaceAll(payload, "../", "")
	payload = strings.ReplaceAll(payload, ".\\", "")
	payload = strings.ReplaceAll(payload, "..\\", "")
	return payload
}
func SanitizeCommandInjection(payload string) string {
	payload = strings.ReplaceAll(payload, "|", "")
	payload = strings.ReplaceAll(payload, "&", "")
	payload = strings.ReplaceAll(payload, ";", "")
	payload = strings.ReplaceAll(payload, "`", "")
	payload = strings.ReplaceAll(payload, "'", "")
	payload = strings.ReplaceAll(payload, "\"", "")
	//payload = strings.ReplaceAll(payload, "\\", "")
	payload = strings.ReplaceAll(payload, "$", "")
	payload = strings.ReplaceAll(payload, "(", "")
	payload = strings.ReplaceAll(payload, ")", "")
	payload = strings.ReplaceAll(payload, "{", "")
	payload = strings.ReplaceAll(payload, "}", "")
	payload = strings.ReplaceAll(payload, "<", "")
	payload = strings.ReplaceAll(payload, ">", "")
	payload = strings.ReplaceAll(payload, "!", "")
	payload = strings.ReplaceAll(payload, "?", "")
	payload = strings.ReplaceAll(payload, "~", "")
	payload = strings.ReplaceAll(payload, "[", "")
	payload = strings.ReplaceAll(payload, "]", "")
	payload = strings.ReplaceAll(payload, "*", "")
	payload = strings.ReplaceAll(payload, "\n", "")
	payload = strings.ReplaceAll(payload, "\r", "")
	return payload
}
func SanitizeNewLines(payload string) string {
	payload = strings.ReplaceAll(payload, "\n", "")
	payload = strings.ReplaceAll(payload, "\r", "")
	return payload
}
func SanitizeLeet(input string) []string {
	// take in a possible 1337 speak string and return all possible unleeted strings
	leetMap := map[string][]string{
		"a": {"4", "@", "/-\\", "/\\", "/~\\", "/*\\", "^", "aye", "ɐ"},
		"b": {"8", "|3", "13", "|8", "|o", "q"},
		"c": {"(", "<", "{", "[", "©", "ɔ"},
		"d": {"|)", "|]", "[)", "|>", "I)", "|}", "|]", "])", "I>", "p"},
		"e": {"3", "£", "€", "ë", "*", "ǝ"},
		"f": {"|=", "ph", "|#", "|\"", "ƒ", "ɟ"},
		"g": {"6", "9", "&", "ƃ"},
		"h": {"#", "/-/", "[-]", "]-[", ")-(", "#", "}-{", "(-)", ")-)", "(-(", ":-:", "|-|", " |-|", "#", "aych", "ɥ"},
		"i": {"!", "1", "|", "eye", "3y3", "ai", "¡", "ᴉ"},
		"j": {"_|", "_/", "¿", "</", "_]", "ʝ", "ɾ"},
		"k": {"|<", "|{", "1<", "l<", "|_", "|c", "ʞ"},
		"l": {"1", "|_", "|", "£", "|", "][", "1_", "l", "|", "el", "!", "l"},
		"m": {"|v|", "|\\/|", "/\\/\\", "/X\\", "|\\|\\", "(u)", "(V)", "(\\/)", "/|\\", "/v\\", "][\\][", "^^", "nn", "IVIV", "nn", "IVI", "IVI", "ɯ"},
		"n": {"|\\|", "/\\/", "/V", "][\\][", "l\\", "|\\|", "/V", "И", "n", "И", "u"},
		"o": {"0", "()", "oh", "[]", "p", "<>", "Ø", "¤", "°", "ø", "o", "*"},
		"p": {"|*", "|o", "|º", "|^(o)", "|>", "p", "|D", "d"},
		"q": {"9", "0_", "kw", "kw", "O_", "[]_", "2", "b"},
		"r": {"|2", "12", ".-", "|^", "l2", "|9", "|2", "2", "®", "[z", "|`", "|~", "|?", "/2", "I2", "|2", "Я", ".-", "ɹ"},
		"s": {"5", "$", "z", "§", "ehs", "es", "s"},
		"t": {"7", "+", "7", "-|-", "1", "']['", "†", "ʇ"},
		"u": {"|_|", "v", "L|", "n"},
		"v": {"\\/", "|/", "\\|", "ʌ", "\\/", "u"},
		"w": {"\\|/", "vv", "\\N", "'//", "\\\\'", "\\^/", "(n)", "\\V/", "\\X/", "\\|/", "ʍ", "\\/\\/", "uu"},
		"x": {"><", "*", "%", ")(", "Ж", "}{", "ecks", "×", "][", "x"},
		"y": {"`/", "\\|/", "¥", "Ч", "7", "%", ">/", "\\//", "ʎ"},
		"z": {"2", "7_", "-/_", "%", ">_", "~/_", "-\\_", "-|_", "z", "zee"},
	}
	results := []string{}
	for _, char := range input {
		if replacements, ok := leetMap[string(char)]; ok {
			for _, replacement := range replacements {
				results = append(results, strings.Replace(input, string(char), replacement, -1))
			}
		}
	}
	return nil
}
func SanitizeNonPrintable(payload string) string {
	if len(payload) == 0 {
		return payload
	}
	// Convert string to rune slice for proper Unicode handling
	runes := []rune(payload)
	result := make([]rune, 0, len(runes))
	// Process each character
	for _, r := range runes {
		// Check if the character is printable using Unicode properties (letters, numbers, punctuation, emojis, symbols and spaces)
		if unicode.IsPrint(r) {
			result = append(result, r)
		}
	}
	return string(result)
}
func SanitizeBOM(payload string) string {
	bom := []byte{0xEF, 0xBB, 0xBF}
	bytesInput := []byte(payload)
	bytesInput = bytes.TrimPrefix(bytesInput, bom)
	return string(bytesInput)
}
func SanitizeReservedTLDs(payload string) string {
	reservedTLDs := []string{"local", "localhost", "onion", "base.eth", "eth", "algo"}
	cleanPayload := ""
	for _, reservedTLD := range reservedTLDs {
		if strings.HasSuffix(payload, "."+reservedTLD) {
			cleanPayload = strings.TrimSuffix(payload, "."+reservedTLD)
		}
	}
	return cleanPayload
}
func ConvertPasskeyToEthSignature(passkeySig []byte) ([]byte, error) {
	if len(passkeySig) < 65 {
		return nil, core.LogErrorReturn("Invalid passkey signature")
	}
	var r, s []byte
	var v byte
	for i := 0; i < len(passkeySig)-65; i++ {
		// Look for a sequence that could be an r,s pair
		potentialR := passkeySig[i : i+32]
		potentialS := passkeySig[i+32 : i+64]
		// Check if this looks like a valid signature component
		if !IsAllZeros(potentialR) && !IsAllZeros(potentialS) {
			r = potentialR
			s = potentialS
			// For v, we typically use 27 or 28
			v = 27
			break
		}
	}
	if r == nil || s == nil {
		return nil, core.LogErrorReturn("Failed to extract r,s from passkey signature")
	}
	// Combine into standard Ethereum signature format
	ethSig := make([]byte, 65)
	copy(ethSig[0:32], r)
	copy(ethSig[32:64], s)
	ethSig[64] = v
	return ethSig, nil
}
