package abugo

/*
	go get github.com/beego/beego/logs
	go get github.com/spf13/viper
	go get github.com/gin-gonic/gin
	go get github.com/go-redis/redis
	go get github.com/garyburd/redigo/redis
	go get github.com/go-sql-driver/mysql
	go get github.com/satori/go.uuid
	go get github.com/gorilla/websocket
	go get github.com/jinzhu/gorm
	go get github.com/imroc/req
	go get github.com/go-playground/validator/v10
	go get github.com/go-playground/universal-translator
	go get code.google.com/p/mahonia
	go get github.com/360EntSecGroup-Skylar/excelize
	go clean -modcache
*/
import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	crand "crypto/rand"
	mrand "math/rand"

	"github.com/beego/beego/logs"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/yinheli/qqwry"
)

var ipdata string
var keyprefix string
var project string
var module string
var env string
var repeat_control sync.Map

func Init() {
	mrand.Seed(time.Now().Unix())
	gin.SetMode(gin.ReleaseMode)
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)
	logs.SetLogger(logs.AdapterFile, `{"filename":"_log/logfile.log","maxsize":10485760}`)
	logs.SetLogger(logs.AdapterConsole, `{"color":true}`)
	viper.AddConfigPath("./")
	viper.AddConfigPath("./config")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		logs.Error(err)
		return
	}
	nodeid := GetConfigInt64("server.snowflakeid", false, 0)
	if nodeid > 0 {
		NewIdWorker(nodeid)
	}
	ipdata = GetConfigString("server.ipdata", false, "")
	project = GetConfigString("server.project", true, "")
	module = GetConfigString("server.module", true, "")
	env = GetConfigString("server.env", true, "")
	keyprefix = fmt.Sprint(project, ":", module, ":")
	repeat_control = sync.Map{}
}

func Run() {
	for {
		time.Sleep(time.Second * 1)
	}
}

func InterfaceToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch v.(type) {
	case string:
		return v.(string)
	case int:
		return fmt.Sprint(v.(int))
	case int32:
		return fmt.Sprint(v.(int32))
	case int64:
		return fmt.Sprint(v.(int64))
	case float32:
		return fmt.Sprint(v.(float32))
	case float64:
		return fmt.Sprint(v.(float64))
	}
	return ""
}

func GetMapString(mp *map[string]interface{}, field string) string {
	if mp == nil {
		return ""
	}
	v := (*mp)[field]
	return InterfaceToString(v)
}

func InterfaceToInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch v.(type) {
	case string:
		i, err := strconv.ParseInt(v.(string), 10, 64)
		if err != nil {
			return 0
		}
		return i
	case int:
		return int64(v.(int))
	case int32:
		return int64(v.(int32))
	case int64:
		return int64(v.(int64))
	case float32:
		return int64(v.(float32))
	case float64:
		return int64(v.(float64))
	}
	return 0
}

func GetMapInt64(mp *map[string]interface{}, field string) int64 {
	if mp == nil {
		return 0
	}
	v := (*mp)[field]
	return InterfaceToInt64(v)
}

func InterfaceToInt(v interface{}) int32 {
	if v == nil {
		return 0
	}
	switch v.(type) {
	case string:
		i, err := strconv.ParseInt(v.(string), 10, 64)
		if err != nil {
			return 0
		}
		return int32(i)
	case int:
		return int32(v.(int))
	case int32:
		return int32(v.(int32))
	case int64:
		return int32(v.(int64))
	case float32:
		return int32(v.(float32))
	case float64:
		return int32(v.(float64))
	}
	return 0
}

func GetMapInt(mp *map[string]interface{}, field string) int32 {
	if mp == nil {
		return 0
	}
	v := (*mp)[field]
	return InterfaceToInt(v)
}

func InterfaceToFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch v.(type) {
	case string:
		i, err := strconv.ParseFloat(v.(string), 64)
		if err != nil {
			return 0
		}
		return i
	case int:
		return float64(v.(int))
	case int32:
		return float64(v.(int32))
	case int64:
		return float64(v.(int64))
	case float32:
		return float64(v.(float32))
	case float64:
		return v.(float64)
	}
	return 0
}

func GetMapFloat64(mp *map[string]interface{}, field string) float64 {
	if mp == nil {
		return 0
	}
	v := (*mp)[field]
	return InterfaceToFloat64(v)
}

func InterfaceToFloat(v interface{}) float32 {
	if v == nil {
		return 0
	}
	switch v.(type) {
	case string:
		i, err := strconv.ParseFloat(v.(string), 64)
		if err != nil {
			return 0
		}
		return float32(i)
	case int:
		return float32(v.(int))
	case int32:
		return float32(v.(int32))
	case int64:
		return float32(v.(int64))
	case float32:
		return float32(v.(float32))
	case float64:
		return float32(v.(float64))
	}
	return 0
}

func GetMapFloat(mp *map[string]interface{}, field string) float32 {
	if mp == nil {
		return 0
	}
	v := (*mp)[field]
	return InterfaceToFloat(v)
}

func GetIpLocation(ip string) string {
	if ipdata == "" {
		return ""
	}
	iptool := qqwry.NewQQwry("./config/ipdata.dat")
	if strings.Index(ip, ".") > 0 {
		iptool.Find(ip)
		return fmt.Sprintf("%s %s", iptool.Country, iptool.City)
	} else {
		return ""
	}
}

func GetGoogleCode(secret string) int32 {
	key, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		logs.Error(err)
		return 0
	}
	epochSeconds := time.Now().Unix() + 0
	return int32(abuOneTimePassword(key, abuToBytes(epochSeconds/30)))
}

func VerifyGoogleCode(secret string, code string) bool {
	nowcode := GetGoogleCode(secret)
	if fmt.Sprint(nowcode) == code {
		return true
	}
	return false
}

func abuOneTimePassword(key []byte, value []byte) uint32 {
	hmacSha1 := hmac.New(sha1.New, key)
	hmacSha1.Write(value)
	hash := hmacSha1.Sum(nil)
	offset := hash[len(hash)-1] & 0x0F
	hashParts := hash[offset : offset+4]
	hashParts[0] = hashParts[0] & 0x7F
	number := abuToUint32(hashParts)
	pwd := number % 1000000
	return pwd
}

func abuToUint32(bytes []byte) uint32 {
	return (uint32(bytes[0]) << 24) + (uint32(bytes[1]) << 16) +
		(uint32(bytes[2]) << 8) + uint32(bytes[3])
}

func abuToBytes(value int64) []byte {
	var result []byte
	mask := int64(0xFF)
	shifts := [8]uint16{56, 48, 40, 32, 24, 16, 8, 0}
	for _, shift := range shifts {
		result = append(result, byte((value>>shift)&mask))
	}
	return result
}

func NewGoogleSecret() string {
	dictionary := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var bytes = make([]byte, 32)
	_, _ = crand.Read(bytes)
	for k, v := range bytes {
		bytes[k] = dictionary[v%byte(len(dictionary))]
	}
	return strings.ToUpper(string(bytes))
}

func aesPKCS7Padding(ciphertext []byte, blocksize int) []byte {
	padding := blocksize - len(ciphertext)%blocksize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}
func aesPKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func AesEncrypt(orig string, key string) string {
	origData := []byte(orig)
	k := []byte(key)
	block, erra := aes.NewCipher(k)
	if erra != nil {
		logs.Error(erra)
		return ""
	}
	blockSize := block.BlockSize()
	origData = aesPKCS7Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, k[:blockSize])
	cryted := make([]byte, len(origData))
	blockMode.CryptBlocks(cryted, origData)
	return base64.StdEncoding.EncodeToString(cryted)
}

func AesDecrypt(cryted string, key string) string {
	crytedByte, _ := base64.StdEncoding.DecodeString(cryted)
	k := []byte(key)
	block, _ := aes.NewCipher(k)
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, k[:blockSize])
	orig := make([]byte, len(crytedByte))
	blockMode.CryptBlocks(orig, crytedByte)
	orig = aesPKCS7UnPadding(orig)
	return string(orig)
}

func RsaSign(strdata string, privatekey string) string {
	privatekey = strings.Replace(privatekey, "-----BEGIN PRIVATE KEY-----", "", -1)
	privatekey = strings.Replace(privatekey, "-----END PRIVATE KEY-----", "", -1)
	privatekey = strings.Replace(privatekey, "-----BEGIN RSA PRIVATE KEY-----", "", -1)
	privatekey = strings.Replace(privatekey, "-----END RSA PRIVATE KEY-----", "", -1)
	privatekeybase64, errb := base64.StdEncoding.DecodeString(privatekey)
	if errb != nil {
		logs.Error(errb)
		return ""
	}
	privatekeyx509, errc := x509.ParsePKCS8PrivateKey([]byte(privatekeybase64))
	if errc != nil {
		logs.Error(errc)
		return ""
	}
	hashmd5 := md5.Sum([]byte(strdata))
	hashed := hashmd5[:]
	sign, errd := rsa.SignPKCS1v15(crand.Reader, privatekeyx509.(*rsa.PrivateKey), crypto.MD5, hashed)
	if errd != nil {
		logs.Error(errd)
		return ""
	}
	return base64.StdEncoding.EncodeToString(sign)
}

func RsaVerify(strdata string, strsign string, publickey string) bool {
	publickey = strings.Replace(publickey, "-----BEGIN PUBLIC KEY-----", "", -1)
	publickey = strings.Replace(publickey, "-----END PUBLIC KEY-----", "", -1)
	publickey = strings.Replace(publickey, "-----BEGIN RSA PUBLIC KEY-----", "", -1)
	publickey = strings.Replace(publickey, "-----END RSA PUBLIC KEY-----", "", -1)
	publickeybase64, errb := base64.StdEncoding.DecodeString(publickey)
	if errb != nil {
		logs.Error(errb)
		return false
	}
	publickeyx509, errc := x509.ParsePKIXPublicKey([]byte(publickeybase64))
	if errc != nil {
		logs.Error(errc)
		return false
	}
	hash := md5.New()
	hash.Write([]byte(strdata))
	signdata, _ := base64.StdEncoding.DecodeString(strsign)
	errd := rsa.VerifyPKCS1v15(publickeyx509.(*rsa.PublicKey), crypto.MD5, hash.Sum(nil), signdata)
	return errd == nil
}

func ReadAllText(path string) string {
	file, err := os.Open(path)
	if err != nil {
		logs.Error(err)
		return ""
	}
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		logs.Error(err)
		return ""
	}
	return string(bytes)
}

// order 排序, asc 升序,desc降序
func SerialObject_v1(data interface{}, order string) string {
	sb := strings.Builder{}
	t := reflect.TypeOf(data)
	v := reflect.ValueOf(data)
	keys := []string{}
	for i := 0; i < t.NumField(); i++ {
		fn := strings.ToLower(t.Field(i).Name)
		if fn != "sign" {
			keys = append(keys, t.Field(i).Name)
		}
	}
	if strings.ToLower(order) == "asc" {
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] < keys[j]
		})
	}
	if strings.ToLower(order) == "desc" {
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] > keys[j]
		})
	}
	for i := 0; i < len(keys); i++ {
		tag, _ := t.FieldByName(keys[i])
		fieldname := tag.Tag.Get("name")
		if fieldname == "" {
			fieldname = keys[i]
		}
		switch sv := v.FieldByName(keys[i]).Interface().(type) {
		case string:
			sb.WriteString(fieldname)
			sb.WriteString("=")
			sb.WriteString(sv)
			sb.WriteString("&")
		case int:
			sb.WriteString(fieldname)
			sb.WriteString("=")
			sb.WriteString(fmt.Sprint(sv))
			sb.WriteString("&")
		case int8:
			sb.WriteString(fieldname)
			sb.WriteString("=")
			sb.WriteString(fmt.Sprint(sv))
			sb.WriteString("&")
		case int16:
			sb.WriteString(fieldname)
			sb.WriteString("=")
			sb.WriteString(fmt.Sprint(sv))
			sb.WriteString("&")
		case int32:
			sb.WriteString(fieldname)
			sb.WriteString("=")
			sb.WriteString(fmt.Sprint(sv))
			sb.WriteString("&")
		case int64:
			sb.WriteString(fieldname)
			sb.WriteString("=")
			sb.WriteString(fmt.Sprint(sv))
			sb.WriteString("&")
		case float32:
			sb.WriteString(fieldname)
			sb.WriteString("=")
			sb.WriteString(fmt.Sprint(sv))
			sb.WriteString("&")
		case float64:
			sb.WriteString(fieldname)
			sb.WriteString("=")
			sb.WriteString(fmt.Sprint(sv))
			sb.WriteString("&")
		}
	}
	str := sb.String()
	if len(str) > 0 {
		str = str[0 : len(str)-1]
	}
	return str
}

func KeyPrefix() string {
	return keyprefix
}

func Project() string {
	return project
}

func Module() string {
	return module
}

func Env() string {
	return env
}

func Md5(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

func TimeStampToLocalDate(tvalue int64) string {
	if tvalue == 0 {
		return ""
	}
	tm := time.Unix(tvalue/1000, 0)
	tstr := tm.Format("2006-01-02")
	return strings.Split(tstr, " ")[0]
}

func LocalDateToTimeStamp(timestr string) int64 {
	t, _ := time.ParseInLocation("2006-01-02", timestr, time.Local)
	return t.Local().Unix()
}

func TimeStampToLocalTime(tvalue int64) string {
	if tvalue == 0 {
		return ""
	}
	tm := time.Unix(tvalue/1000, 0)
	tstr := tm.Format("2006-01-02 15:04:05")
	return tstr
}

func LocalTimeToTimeStamp(timestr string) int64 {
	t, _ := time.ParseInLocation("2006-01-02 15:04:05", timestr, time.Local)
	return t.Local().Unix()
}

func GetUtcTimeStamp(timestr string) string {
	if len(timestr) == 0 {
		return timestr
	}
	if len(timestr) == 10 {
		timestr = timestr + " 00:00:00"
	}
	t, _ := time.ParseInLocation("2006-01-02 15:04:05", timestr, time.Local)
	r := t.UTC().Format("2006-01-02T15:04:05Z")
	return r
}

func GetRepeatControl(key string) bool {
	_, under_control := repeat_control.Load(key)
	if under_control {
		return false
	}
	repeat_control.Store(key, 1)
	return true
}

func ReleaseRepeatControl(key string) {
	repeat_control.Delete(key)
}
