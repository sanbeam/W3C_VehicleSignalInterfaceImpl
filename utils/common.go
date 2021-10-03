/**
* (C) 2021 Geotab Inc
*
* All files and artifacts in the repository at https://github.com/MEAE-GOT/W3C_VehicleSignalInterfaceImpl
* are licensed under the provisions of the license provided by the LICENSE file in this repository.
*
**/

package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/binary"
	"bytes"
	"net"
	"os"
	"strings"
	"strconv"
	"io/ioutil"
        "time"
        "sort"
        "fmt"
)

const IpModel = 0 // IpModel = [0,1,2] = [localhost,extIP,envVarIP]
const IpEnvVarName = "GEN2MODULEIP"

func GetServerIP() string {
	if value, ok := os.LookupEnv(IpEnvVarName); ok {
		Info.Println("ServerIP:", value)
		return value
	}
	Error.Printf("Environment variable %s is not set defaulting to localhost.", IpEnvVarName)
	return "localhost" //fallback
}

func GetModelIP(ipModel int) string {
	if ipModel == 0 {
		return "localhost"
	}
	if ipModel == 2 {
		if value, ok := os.LookupEnv(IpEnvVarName); ok {
			Info.Println("Host IP:", value)
			return value
		}
		Error.Printf("Environment variable %s error.", IpEnvVarName)
		return "localhost" //fallback
	}
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		Error.Fatal(err.Error())
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	Info.Println("Host IP:", localAddr.IP)

	return localAddr.IP.String()
}

func MapRequest(request string, rMap *map[string]interface{}) int {
	decoder := json.NewDecoder(strings.NewReader(request))
	err := decoder.Decode(rMap)
	if err != nil {
		Error.Printf("extractPayload: JSON decode failed for request:%s\n", request)
		return -1
	}
	return 0
}

func UrlToPath(url string) string {
	var path string = strings.TrimPrefix(strings.Replace(url, "/", ".", -1), ".")
	return path[:]
}

func PathToUrl(path string) string {
	var url string = strings.Replace(path, ".", "/", -1)
	return "/" + url
}

func GenerateHmac(input string, key string) string { //not a correct JWT signature?
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(input))
	return string(mac.Sum(nil))
}

func VerifyTokenSignature(token string, key string) bool { // compatible with result from generateHmac()
	delimiter := strings.LastIndex(token, ".")
	if delimiter == -1 {
		return false
	}
	message := token[:delimiter]
	messageMAC := token[delimiter+1:]
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(message))
	expectedMAC := mac.Sum(nil)
	if strings.Compare(messageMAC, base64.RawURLEncoding.EncodeToString(expectedMAC)) == 0 {
		return true
	}
	return false
}

func ExtractFromToken(token string, claim string) string { // TODO remove white space sensitivity
	delimiter1 := strings.Index(token, ".")
	delimiter2 := strings.Index(token[delimiter1+1:], ".") + delimiter1 + 1
	header := token[:delimiter1]
	payload := token[delimiter1+1 : delimiter2]
	decodedHeaderByte, _ := base64.RawURLEncoding.DecodeString(header)
	decodedHeader := string(decodedHeaderByte)
	claimIndex := strings.Index(decodedHeader, claim)
	if claimIndex != -1 {
		startIndex := claimIndex + len(claim) + 2
		endIndex := strings.Index(decodedHeader[startIndex:], ",") + startIndex // ...claim":abc,...  or ...claim":"abc",... or See next line
		if endIndex == startIndex-1 {                                           // ...claim":abc}  or ...claim":"abc"}
			endIndex = len(decodedHeader) - 1
		}
		if string(decodedHeader[endIndex-1]) == `"` {
			endIndex--
		}
		if string(decodedHeader[startIndex]) == `"` {
			startIndex++
		}
		return decodedHeader[startIndex:endIndex]
	}
	decodedPayloadByte, _ := base64.RawURLEncoding.DecodeString(payload)
	decodedPayload := string(decodedPayloadByte)
	claimIndex = strings.Index(decodedPayload, claim)
	if claimIndex != -1 {
		startIndex := claimIndex + len(claim) + 2
		endIndex := strings.Index(decodedPayload[startIndex:], ",") + startIndex // ...claim":abc,...  or ...claim":"abc",... or See next line
		if endIndex == startIndex-1 {                                            // ...claim":abc}  or ...claim":"abc"}
			endIndex = len(decodedPayload) - 1
		}
		if string(decodedPayload[endIndex-1]) == `"` {
			endIndex--
		}
		if string(decodedPayload[startIndex]) == `"` {
			startIndex++
		}
		return decodedPayload[startIndex:endIndex]
	}
	return ""
}

func SetErrorResponse(reqMap map[string]interface{}, errRespMap map[string]interface{}, number string, reason string, message string) {
	if reqMap["RouterId"] != nil {
		errRespMap["RouterId"] = reqMap["RouterId"]
	}
	if reqMap["action"] != nil {
		errRespMap["action"] = reqMap["action"]
	}
	if reqMap["requestId"] != nil {
		errRespMap["requestId"] = reqMap["requestId"]
	}
	errRespMap["error"] = `{"number":` + number + `,"reason":"` + reason + `","message":"` + message + `"}`
        errRespMap["ts"] = GetRfcTime()
}

func FinalizeMessage(responseMap map[string]interface{}) string {
	response, err := json.Marshal(responseMap)
	if err != nil {
		Error.Print("Server core-FinalizeMessage: JSON encode failed. ", err)
		return `{"error":{"number":400,"reason":"JSON marshal error","message":""}}` //???
	}
	return string(response)
}

func AddKeyValue(message string, key string, value string) string {  // to avoid Marshal() to reformat using \" 
    if (len(value) > 0) {
        if (value[0] == '{') {
            return message[:len(message)-1] + ", \"" + key + "\":" + value + "}"
        } 
        return message[:len(message)-1] + ", \"" + key + "\":\"" + value + "\"}"
    }
    return message
}

func GetRfcTime() string {
    withTimeZone := time.Now().Format(time.RFC3339)   // 2020-05-01T15:34:35+02:00
    if (withTimeZone[len(withTimeZone)-6] == '+') {
        return withTimeZone[:len(withTimeZone)-6] + "Z"
    } else {
        return withTimeZone
    }
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

type FilterObject struct {
    Type string
    Value string
}

func UnpackFilter(filter interface{}, fList *[]FilterObject) {  // See VISSv CORE, Filtering chapter for filter structure
    switch vv := filter.(type) {
      case []interface{}:
        Info.Println(filter, "is an array:, len=",strconv.Itoa(len(vv)))
        *fList = make([]FilterObject, len(vv))
  	unpackFilterLevel1(vv, fList)
      case map[string]interface{}:
        Info.Println(filter, "is a map:")
        *fList = make([]FilterObject, 1)
        unpackFilterLevel2(0, vv, fList)
      default:
        Info.Println(filter, "is of an unknown type")
    }
}

func unpackFilterLevel1(filterArray []interface{}, fList *[]FilterObject) {
    i := 0
    for k, v := range filterArray {
        switch vv := v.(type) {
          case map[string]interface{}:
            Info.Println(k, "is a map:")
            unpackFilterLevel2(i, vv, fList)
          default:
            Info.Println(k, "is of an unknown type")
        }
        i++
    }
}

func unpackFilterLevel2(index int, filterExpression map[string]interface{}, fList *[]FilterObject) {
    for k, v := range filterExpression {
        switch vv := v.(type) {
          case string:
            Info.Println(k, "is string", vv)
            if (k == "type") {
                (*fList)[index].Type = vv
            } else if (k == "value") {
                (*fList)[index].Value = vv
            }
          case []interface{}:
            Info.Println(k, "is an array:, len=",strconv.Itoa(len(vv)))
            arrayVal, err := json.Marshal(vv)
            if err != nil {
		Error.Print("UnpackFilter(): JSON array encode failed. ", err)
	    } else if (k == "value") {
	        (*fList)[index].Value = string(arrayVal)
	    }
          case map[string]interface{}:
            Info.Println(k, "is a map:")
            opValue, err := json.Marshal(vv)
            if err != nil {
		Error.Print("UnpackFilter(): JSON map encode failed. ", err)
	    } else {
	        (*fList)[index].Value = string(opValue)
	    }
          default:
            Info.Println(k, "is of an unknown type")
        }
    }
}

/**************** Compression reference implementation ***********************/

func NextQuoteMark(message []byte, offset int) int {
    for i := offset ; i < len(message) ; i++ {
        if (message[i] == '"') {
            return i
        }
    }
    return offset
}

func decompressPath(index []byte) []byte {
            path := "\""
            i := int(index[0])*256 + int(index[1])
            path += pathList.Path[i]
            path += "\""
            return []byte(path)
}

func expandTsItem(tsItem byte,index int) []byte { //yyyy-mm-ddThh:mm:ss<.ssss>Z  TODO: support for subsec
    expandedItem := make([]byte, 3)
    expandedItem[0] = tsItem/10 + '0'
    expandedItem[1] = tsItem%10 + '0'
    if (index < 2) {
        expandedItem[2] = '-'
    } else if (index == 2) {
        expandedItem[2] = 'T'
    } else if (index > 2 && index < 5) {
        expandedItem[2] = ':'
    } else {
        expandedItem[2] = 'Z'
    }
    return expandedItem
}

func decompressTs(tsCompressed []byte) []byte {
//Info.Printf("tsCompressed[0]=%d, tsCompressed[1]=%d, tsCompressed[2]=%d, tsCompressed[3]=%d", tsCompressed[0], tsCompressed[1], tsCompressed[2], tsCompressed[3])
    tsUncompressed := make([]byte, 22)
    tsUncompressed[0] = '"'
    tsUncompressed[1] = '2'
    tsUncompressed[2] = '0'
    tsUncompressed[3] = '2'  // TODO: get the three MSDigit(year) from system clock
    tsUncompressed[4] = tsCompressed[0] / 4 + '0'
    tsUncompressed[5] = '-'
    tsUncompressed[6] = ((tsCompressed[1] & 0xC0) / 64 + (tsCompressed[0] & 0x3) * 4) / 10 + '0'
    tsUncompressed[7] = ((tsCompressed[1] & 0xC0) / 64 + (tsCompressed[0] & 0x3) * 4) % 10 + '0'
    tsUncompressed[8] = '-'
    tsUncompressed[9] = ((tsCompressed[1] & 0x3E) / 2) / 10 + '0'
    tsUncompressed[10] = ((tsCompressed[1] & 0x3E) / 2) % 10 + '0'
    tsUncompressed[11] = 'T'
    tsUncompressed[12] = ((tsCompressed[2] & 0xF0) / 16 + (tsCompressed[1] & 0x1) * 16) / 10 + '0'
    tsUncompressed[13] = ((tsCompressed[2] & 0xF0) / 16 + (tsCompressed[1] & 0x1) * 16) % 10 + '0'
    tsUncompressed[14] = ':'
    tsUncompressed[15] = ((tsCompressed[3] & 0xC0) / 64 + (tsCompressed[2] & 0x0F) * 4) / 10 + '0'
    tsUncompressed[16] = ((tsCompressed[3] & 0xC0) / 64 + (tsCompressed[2] & 0x0F) * 4) % 10 + '0'
    tsUncompressed[17] = ':'
    tsUncompressed[18] = ((tsCompressed[3] & 0x3F) / 10) + '0'
    tsUncompressed[19] = ((tsCompressed[3] & 0x3F) % 10) + '0'
    tsUncompressed[20] = 'Z'
    tsUncompressed[21] = '"'
    return tsUncompressed
}

func convert4BytesToF32(byteBuf []byte) float32 {
       byte4Buf := make([]byte, 4)
       byte4Buf[0] = byteBuf[0]
       byte4Buf[1] = byteBuf[1]
       byte4Buf[2] = byteBuf[2]
       byte4Buf[3] = byteBuf[3]
       var f32Val float32
	buf := bytes.NewReader(byte4Buf)
	err := binary.Read(buf, binary.LittleEndian, &f32Val)
	if err != nil {
		Error.Println("binary.Read failed:", err)
	}
	return f32Val
}

func decompressOneValue(valueCompressed []byte) ([]byte, int) {
    var unCompressedValue []byte
    var bytesRead int
    valueLead := "\""
    switch codeList.Code[valueCompressed[0]-128] {
      case "nuint8":
        valueLead += "-"
        fallthrough
      case "uint8":
        value := valueLead + strconv.Itoa(int(valueCompressed[1])) + "\""
        unCompressedValue = append(unCompressedValue, []byte(value)...)
        bytesRead = 2
      case "nuint16":
        valueLead += "-"
        fallthrough
      case "uint16":
        value := valueLead + strconv.Itoa(int(valueCompressed[1])*256+int(valueCompressed[2])) + "\""
        unCompressedValue = append(unCompressedValue, []byte(value)...)
        bytesRead = 3
      case "nuint24":
        valueLead += "-"
        fallthrough
      case "uint24":
        value := valueLead + strconv.Itoa(int(valueCompressed[1])*65536+int(valueCompressed[2])*256+int(valueCompressed[3])) + "\""
        unCompressedValue = append(unCompressedValue, []byte(value)...)
        bytesRead = 4
      case "nuint32":
        valueLead += "-"
        fallthrough
      case "uint32":
        value := valueLead + strconv.Itoa(int(valueCompressed[1])*16777216+int(valueCompressed[2])*65536+int(valueCompressed[3])*256+int(valueCompressed[4])) + "\""
        unCompressedValue = append(unCompressedValue, []byte(value)...)
        bytesRead = 5
      case "bool":
        if (valueCompressed[1] == 0) {
            unCompressedValue = append(unCompressedValue, []byte("\"false\"")...)
        } else {
            unCompressedValue = append(unCompressedValue, []byte("\"true\"")...)
        }
        bytesRead = 2
      case "float":
        f32Val := convert4BytesToF32(valueCompressed[1:5])
        value := "\"" + fmt.Sprintf("%f", f32Val) + "\""
        unCompressedValue = append(unCompressedValue, []byte(value)...)
        bytesRead = 5
//      case "unknown":  handled by default
      default:
        bytesRead = strings.Index(string(valueCompressed[3:]), "\"") + 4
        unCompressedValue = valueCompressed[1:bytesRead]
    }
    return unCompressedValue, bytesRead
}

func decompressValue(valueCompressed []byte) ([]byte, int) {
    var unCompressedValue []byte
    var unCompressedOneValue []byte
    nonValue := make([]byte, 1)
    var bytesRead int
    index := 0
    isDone := false
    for isDone != true {
        if (valueCompressed[index] == '[' || valueCompressed[index] == ']' || valueCompressed[index] == ',' || valueCompressed[index] == '{') {
            nonValue[0] = valueCompressed[index]
            unCompressedValue = append(unCompressedValue, nonValue...)
            if (valueCompressed[index] == ']' || valueCompressed[index] == '{') {
                isDone = true
            }
            index += 1
        } else {
            unCompressedOneValue, bytesRead = decompressOneValue(valueCompressed[index:])
            unCompressedValue = append(unCompressedValue, unCompressedOneValue...)
            if (index+bytesRead < len(valueCompressed) && valueCompressed[index+bytesRead] == valueCompressed[index]) {
                nonValue[0] = ','
                unCompressedValue = append(unCompressedValue, nonValue...)
            }
            if (index == 0) {
                isDone = true
            }
            index += bytesRead
        }
    }
    return unCompressedValue, index
}

func readCompressedMessage(message []byte, offset int) ([]byte, int) {
    var unCompressedToken []byte
    extraByte := make([]byte, 1)
    bytesRead := 1
    if (message[offset] >= 128) {
        if (message[offset]-128 < CODELISTKEYVALUES) {
            extraByte[0] = '"'  // quote
            unCompressedToken = append(unCompressedToken, extraByte...)
            unCompressedToken = append(unCompressedToken, []byte(codeList.Code[message[offset]-128])...)
            unCompressedToken = append(unCompressedToken, extraByte...)
            if (message[offset] < CODELISTKEYS + 128) { // this is a key, so a colon should follow
                extraByte[0] = ':'
                unCompressedToken = append(unCompressedToken, extraByte...)
            }
        }
        if (message[offset]-128 == CODELISTINDEXPATH) {
            unCompressedToken = append(unCompressedToken, decompressPath(message[offset+1:])...)
            bytesRead += 2
        } else if (message[offset]-128 == getCodeListIndex("ts")) {
            unCompressedToken = append(unCompressedToken, decompressTs(message[offset+1:])...)
            bytesRead += 4
        } else if (message[offset]-128 == CODELISTINDEXVALUE || message[offset]-128 == CODELISTINDEXREQID || message[offset]-128 == CODELISTINDEXSUBID) {
            value, bytes := decompressValue(message[offset+1:])
            unCompressedToken = append(unCompressedToken, value...)
            bytesRead += bytes
        }
        if (message[offset]-128 < CODELISTKEYVALUES && message[offset]-128 != getCodeListIndex("action") && 
            message[offset]-128 != getCodeListIndex("filter") && message[offset]-128 != getCodeListIndex("authorization") &&
            offset + bytesRead != len(message) && message[offset+bytesRead] != '}' && message[offset+bytesRead] != ']' && 
            message[offset+bytesRead-1] != '{' && message[offset+bytesRead-1] != '[') {
//Info.Printf("readCompressedMessage():offset=%d, bytesRead=%d, message[offset:]=%s", offset, bytesRead, string(message[offset:]))
          extraByte[0] = ','
          unCompressedToken = append(unCompressedToken, extraByte...)
        }
    } else {
        extraByte[0] = message[offset]
        unCompressedToken = append(unCompressedToken, extraByte...)
        if (offset+1 < len(message) && message[offset] == '}' && message[offset+1] == '{') {
        extraByte[0] = ','
        unCompressedToken = append(unCompressedToken, extraByte...)
        }
    }
//Info.Printf("readCompressedMessage():offset=%d, bytesRead=%d, unCompressedToken=%s", offset, bytesRead, string(unCompressedToken))
    return unCompressedToken, bytesRead
}

func DecompressMessage(message []byte) []byte {
    var message2 []byte
    curlyBrace := make([]byte, 1)
    if (len(message) == 0) {
        return message
    }
    if (len(codeList.Code) == 0) {
        jsonToStructList(codelist, &codeList)
    }
    if (len(pathList.Path) == 0) {
        numOfPaths := createPathList("../vsspathlist.json") // assuming that the file is in the server directory...
        Info.Printf("Pathlist elements=%d\n", numOfPaths)
    }
    if (message[0] != '{') {
        curlyBrace[0] = '{'
        message2 = append(message2, curlyBrace...)
    }
    for offset := 0 ; offset < len(message) ; {
        uncompressedToken, compressedLen := readCompressedMessage(message, offset)
        offset += compressedLen
        message2 = append(message2, uncompressedToken...)
    }
    if (message[len(message)-1] != '}') {
        curlyBrace[0] = '}'
        message2 = append(message2, curlyBrace...)
    }
    return message2
}

func readUncompressedMessage(message []byte, offset int) []byte {
    var token []byte
    if (message[offset] == '"') {
        offset2 := NextQuoteMark(message, offset+1)
        token = message[offset:offset2+1]
    } else {
        token = []byte(string(message[offset]))
    }
    return token
}

func getCodeListIndex(token string) byte {
    var i byte
    listLen := byte(len(codeList.Code))
    for i = 0 ; i < listLen ; i++ {
//Info.Printf("codeList.Code[%d]=%s, token=%s", i, codeList.Code[i], token)
        if (codeList.Code[i] == token) {
            return i
        }
    }
    return 255
}

func compressPath(path []byte) []byte {
    comparePath := string(path)
    index := sort.Search(len(pathList.Path), func(i int) bool { return comparePath[1:len(comparePath)-1] <= pathList.Path[i] })
    if index < len(pathList.Path) && pathList.Path[index] == comparePath[1:len(comparePath)-1] {
        Info.Printf("Found %s at index %d.", path, index)
        listIndex := make([]byte, 2)
        listIndex[0] = byte((index & 0xFF00)/256)
        listIndex[1] = byte(index & 0x00FF)
        return listIndex
    } else {
        Info.Printf("Did not find %s", path)
        return path
    }
}

func twoToOneByte(twoByte []byte) byte {
    var oneByte byte
    oneByte = (twoByte[0]-48)*10 + (twoByte[1]-48)  // decimal ASCII value for zero = 48
    return oneByte
}

func compressTS(ts []byte) []byte {  // ts = "YYYY-MM-DDTHH:MM:SS<.sss...>Z", LSDigit(year) => 4 bits, month=>4 bits, day=>5 bits, hour=>5 bits, minute=>6 bits, second=>6 bits
    compressedTs := make([]byte, 4)

    second := twoToOneByte(ts[18:20])
    minute := twoToOneByte(ts[15:17])
    hour := twoToOneByte(ts[12:14])
    day := twoToOneByte(ts[9:11])
    month := twoToOneByte(ts[6:8])
    year := ts[4] - '0'
//Info.Printf("year=%d, month=%d, day=%d, hour=%d, minute=%d, second=%d", year, month, day, hour, minute, second)
    compressedTs[3] = (minute & 0x3)*64 + second  // 2 LSB from minute, and 6 bits from second
    compressedTs[2] = (hour & 0xF)*16 + (minute / 4) // 4 LSB from hour, and 4 MSB from minute
    compressedTs[1] = (month & 0x3)*64 + (day * 2) + (hour / 16) // 2 LSB from month, 5 bits from day, and 1 MSB from hour
    compressedTs[0] = (year * 4) + (month / 4) // 4 bits from year, and 2 MSB from month
//Info.Printf("compressedTs[0]=%d, compressedTs[1]=%d, compressedTs[2]=%d, compressedTs[3]=%d", compressedTs[0], compressedTs[1], compressedTs[2], compressedTs[3])
//    subsecond := ts[20:len(ts)-2]  TODO: handle subsecond
    return compressedTs
}

func getIntType(byteSize int, isPos bool) string {
    if (byteSize == 1) {
        if (isPos == true) {
            return "uint8"
        }
        return "nuint8"
    }
    if (byteSize == 2) {
        if (isPos == true) {
            return "uint16"
        }
        return "nuint16"
    }
    if (byteSize == 3) {
        if (isPos == true) {
            return "uint24"
        }
        return "nuint24"
    }
    if (isPos == true) {
        return "uint32"
    }
    return "nuint32"
}

func compressIntValue(value []byte) []byte {
    intVal, _ := strconv.Atoi(string(value[1:len(value)-1]))
    isPos := true
    if (intVal < 0) {
        isPos = false
        intVal = intVal * -1
    }
    if (intVal < 256) { // nuint8/uint8
        compressedVal := make([]byte, 2)
        compressedVal[0] = getCodeListIndex(getIntType(1, isPos))+128
        compressedVal[1] = byte(intVal)
        return compressedVal
    } else if (intVal < 65536) {  // nuint16/uint16
        compressedVal := make([]byte, 3)
        compressedVal[0] = getCodeListIndex(getIntType(2, isPos))+128
        compressedVal[1] = byte((intVal & 0xFF00)/256)
        compressedVal[2] = byte(intVal & 0x00FF)
        return compressedVal
    } else if (intVal < 16777216) {  // nuint24/uint24
        compressedVal := make([]byte, 4)
        compressedVal[0] = getCodeListIndex(getIntType(3, isPos))+128
        compressedVal[1] = byte((intVal & 0xFF0000)/65536)
        compressedVal[2] = byte((intVal & 0xFF00)/256)
        compressedVal[3] = byte(intVal & 0x00FF)
        return compressedVal
    } else if (intVal < 4294967296) {  // nuint32/uint32
        compressedVal := make([]byte, 5)
        compressedVal[0] = getCodeListIndex(getIntType(4, isPos))+128
        compressedVal[1] = byte((intVal & 0xFF000000)/16777216)
        compressedVal[2] = byte((intVal & 0xFF0000)/65536)
        compressedVal[3] = byte((intVal & 0xFF00)/256)
        compressedVal[4] = byte(intVal & 0x00FF)
        return compressedVal
    }
    return value // int64 will stay uncoded
}

func compressBoolValue(value []byte) []byte {
        compressedVal := make([]byte, 2)
        compressedVal[0] = getCodeListIndex("bool") + 128
        compressedVal[1] = byte(0)
        if (string(value) == "\"true\"") {
            compressedVal[1] = byte(1)
        }
        return compressedVal
}

func float32ToByte(f32Val float32) []byte {
    buf := new(bytes.Buffer)
    err := binary.Write(buf, binary.LittleEndian, f32Val)
    if err != nil {
	Error.Println("binary.Write failed:", err)
        return buf.Bytes()   // ???
    }
    Info.Printf("Float32=%f, Float32 bytes: %x", f32Val, buf.Bytes())
    return buf.Bytes()
}

func compressFloatValue(value []byte) []byte {
        compressedVal := make([]byte, 5)
        compressedVal[0] = getCodeListIndex("float") + 128
        f64Val, _ := strconv.ParseFloat(string(value[1:len(value)-1]), 32)
        f32Val := float32(f64Val)
        buf := float32ToByte(f32Val)
        compressedVal[1] = buf[0]
        compressedVal[2] = buf[1]
        compressedVal[3] = buf[2]
        compressedVal[4] = buf[3]
        return compressedVal
}

func compressOtherValue(value []byte) []byte {
    var compressedValue []byte
    valueTypeEncoding := make([]byte, 1)
    valueTypeEncoding[0] = getCodeListIndex("unknown") + 128
    compressedValue = append(compressedValue, valueTypeEncoding...)
    compressedValue = append(compressedValue, []byte(value)...)
    return compressedValue
}

func isFloatType(value string) bool {
    fVal, err := strconv.ParseFloat(value, 32)
    if err != nil {
        return false
    }
    if (fVal < 0) {
        fVal = fVal * -1.0
    }
    if (fVal < 1.175494351E-38 || fVal > 3.402823466E+38) { // f64 not supported
        return false
    }
    return true
}

func AnalyzeValueType(value string) int {
    _, err := strconv.Atoi(value)
    if (err == nil) {
        return 1  //int type
    }
    if (value == "true" || value == "false") {
        return 2 // bool type
    }
    if (isFloatType(value) == true) {
        return 3 // float type
    }
    return 0
}

func compressValue(value []byte) []byte {
    var compressedValue []byte

    switch AnalyzeValueType(string(value[1:len(value)-1])) {
      case 1: // int type
        compressedValue = append(compressedValue, compressIntValue(value)...)
      case 2: // bool type
        compressedValue = append(compressedValue, compressBoolValue(value)...)
      case 3: // float32 type
        compressedValue = append(compressedValue, compressFloatValue(value)...)
      case 0: // any other type
        compressedValue = append(compressedValue, compressOtherValue(value)...)
    }
//Info.Printf("analyzeValueType()=%d, value=%s", analyzeValueType(string(value[1:len(value)-1])), string(value))
    return compressedValue
}

func createPathList(fname string) int {
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		Error.Printf("Error reading %s: %s", fname, err)
		return 0
	}
	jsonToStructList(string(data), &pathList)
	return len(pathList.Path)
}

func CompressMessage(message []byte) []byte {
    var message2 []byte
    if (len(codeList.Code) == 0) {
        jsonToStructList(codelist, &codeList)
    }
    if (len(pathList.Path) == 0) {
        numOfPaths := createPathList("../vsspathlist.json") // assuming that the file is in the server directory...
        Info.Printf("Path list elements=%d\n", numOfPaths)
    }
    var tokenState byte
    tokenState = 255
    isArray := false
    for offset := 0 ; offset < len(message) ; {
        token := readUncompressedMessage(message, offset)
//Info.Printf("Token=%s, len=%d", string(token), len(token))
        offset += len(token)
        if (len(token) == 1) {
            if (token[0] != ' ' && token[0] != ',') {  // remove space and comma
                if ((token[0] == '{' && offset == 1) || (token[0] == '}' && offset == len(message)) || (token[0] == ':')) { //remove leading/trailing curly braces, and colon
                    continue
                }
                if (token[0] == '[') {
                    isArray = true
                }
                if (token[0] == ']') {
                    isArray = false
                }
                message2 = append(message2, token...)
            }
        } else {
            listIndex := getCodeListIndex(string(token[1:len(token)-1]))
            listLen := byte(len(codeList.Code))
            if (listIndex < listLen) {
                index := make([]byte, 1)
                index[0] = listIndex + 128
                message2 = append(message2, index...)
                if (listIndex == CODELISTINDEXTS || listIndex == CODELISTINDEXVALUE || listIndex == CODELISTINDEXPATH || 
                    listIndex == CODELISTINDEXREQID || listIndex == CODELISTINDEXSUBID) {
                    tokenState = listIndex
                    isArray = false
                }
            } else {
                if (tokenState == CODELISTINDEXTS) {
                    message2 = append(message2, compressTS(token)...)
                    tokenState = 255
                } else if (tokenState == CODELISTINDEXVALUE || tokenState == CODELISTINDEXREQID || tokenState == CODELISTINDEXSUBID) {
                    message2 = append(message2, compressValue(token)...)
                    if (isArray == false) {
                    tokenState = 255
                    }
                } else if (tokenState == CODELISTINDEXPATH) {
                    message2 = append(message2, compressPath(token)...)
                    tokenState = 255
                } else {
                    message2 = append(message2, token...)
                    if (message[offset] == ':') {
                        colon := make([]byte, 1)
                        colon[0] = ':'
                        message2 = append(message2, colon...) 
Info.Printf("CompressMessage:colon added, token=%s", string(token))
                   }
                }
            }
        }
    }
    for i := 0 ; i < len(message2) ; i++ {
        Info.Printf("mess[%d]=%d,", i, message2[i])
    }
    Info.Printf("Decompressed message=%s, length=%d", DecompressMessage(message2), len(DecompressMessage(message2)))
    Info.Printf("Length of compressed message=%d, ratio =%d%", len(message2), len(DecompressMessage(message2))*100/len(message2))
    return message2
}

/*
* The codelist shall contain all keys used in JSON payloads, all "constant" key values, and number types.
  If the list is extended, keys shall be placed in front of the list, 
* constant key values in the middle of the lst, and number types at the end of the list.
* The CODELISTDELIM must be updated to the correct element numbers.
*/
var codelist string = `{"codes":["action", "requestId", "value", "ts", "path", "subscriptionId", "data", "dp", "filter", "authorization",
                        "get", "set", "subscribe", "unsubscribe", "subscription", 
                        "nuint8", "uint8", "nuint16", "uint16", "nuint24", "uint24", "nuint32", "uint32", "bool", "float", "unknown"]}`

const CODELISTINDEXREQID = 1  // must be set to the list index of the "requestId" element
const CODELISTINDEXVALUE = 2  // must be set to the list index of the "value" element
const CODELISTINDEXTS = 3  // must be set to the list index of the "ts" element
const CODELISTINDEXPATH = 4  // must be set to the list index of the "path" element
const CODELISTINDEXSUBID = 5  // must be set to the list index of the "subscriptionId" element
const CODELISTKEYS = 10  // must be set to the number of keys in the list
const CODELISTKEYVALUES = 15  // must be set to the number of keys plus values in the list (excl value types)

type CodeList struct {
	Code []string `json:"codes"`
}

var codeList CodeList

type PathList struct {
	Path []string `json:"LeafPaths"`
}

var pathList PathList

func jsonToStructList(jsonList string, list interface{}) {
	err := json.Unmarshal([]byte(jsonList), list)
	if err != nil {
		Error.Printf("Error unmarshal json=%s\n", err)
	}
}

