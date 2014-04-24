package main

/* kctool
master file name (need)
api_start2.json

load type name
port
slotitem (need port.json)
master
*/
import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	ship3Exp1      = `=IF(F%[1]d>49,ROUNDUP((F%[1]d-49)/3),"")`
	ship3Exp2      = `=IF(F%[1]d<49,49-F%[1]d,"")`
	masterFileName = "api_start2.json"
)

var (
	dump       bool
	load       string
	typeTable  = make(map[int]string)
	shipTable  = make(map[int]ship)
	equipTable = make(map[int]string)
	itemTable  = make(map[int]string)
	dispItems  = []string{"api_lv", "api_cond", "api_exp", "api_ndock_item", "api_ndock_time"}
)

type ship struct {
	stype int
	name  string
}

func init() {
	flag.BoolVar(&dump, "dump", false, "dump row data")
	flag.StringVar(&load, "load", "port", "load data type")
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func padding(level int) string {
	return strings.Repeat(" ", (level-1)*2)
}

func output(w io.Writer, format string, a ...interface{}) {
	fmt.Fprintf(w, format, a...)
}

func sortedKeys(m map[string]interface{}) []string {
	var keys []string
	for k, _ := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func parse(ofile io.Writer, body interface{}, kname string, level int) {
	level++
	switch v := body.(type) {
	case map[string]interface{}:
		log.Println(kname, "map[string]interface{}")
		output(ofile, "%s%v {\n", padding(level), kname)
		for _, k := range sortedKeys(v) {
			parse(ofile, v[k], k, level)
		}
		output(ofile, "%s}\n", padding(level))
	case []interface{}:
		log.Println(kname, "slice")
		output(ofile, "%s%v [\n", padding(level), kname)
		level++
		for i, s := range v {
			parse(ofile, s, "["+strconv.Itoa(i)+"]", level)
		}
		level--
		output(ofile, "%s]\n", padding(level))
	default:
		log.Println(kname, "primitive")
		output(ofile, "%s%v = %v\n", padding(level), kname, v)
	}
}

func toInt(value interface{}) int {
	return int(value.(float64))
}

func readJson(sname string, proc func(key string, item interface{})) {
	for k, v := range unmarshal(sname) {
		proc(k, v)
	}
}

// api_start2.json の読み込みに特化。api_dataの下にさらに各項目があることが前提
func loadMasterFile(kname string, proc func(item map[string]interface{})) {
	readJson(masterFileName, func(k string, v interface{}) {
		log.Println(k)
		if k == "api_data" {
			for kk, vv := range v.(map[string]interface{}) {
				if kk == kname {
					for _, x := range vv.([]interface{}) {
						proc(x.(map[string]interface{}))
					}
				}
			}
		}
	})
}

func loadTypeTable() {
	loadMasterFile("api_mst_stype", func(item map[string]interface{}) {
		typeTable[toInt(item["api_id"])] = item["api_name"].(string)
	})
	log.Println("load 'stype from api_start2.json'")
}

func loadShipTable() {
	loadMasterFile("api_mst_ship", func(item map[string]interface{}) {
		n := item["api_name"].(string)
		if n == "なし" {
			return
		}
		val := ship{stype: toInt(item["api_stype"]), name: n}
		shipTable[toInt(item["api_id"])] = val
	})
	log.Println("load 'ship master from api_start2.json'")
}

func loadItemTable() {
	loadMasterFile("api_mst_slotitem", func(item map[string]interface{}) {
		id := toInt(item["api_id"])
		name := item["api_name"].(string)
		itemTable[id] = name
	})
	log.Println("load 'item master from api_start2.json'")
}

type itemlist [][]string

func (s itemlist) Len() int {
	return len(s)
}

func (s itemlist) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s itemlist) Less(i, j int) bool {
	if atoi(s[i][0]) < atoi(s[j][0]) {
		return true
	}
	if atoi(s[i][0]) > atoi(s[j][0]) {
		return false
	}
	return atoi(s[i][1]) < atoi(s[j][1])
}

func slotItem(ofile io.Writer, fname string) {
	output(ofile, "ID,ITEMID,名前,装備艦\n")
	odata := [][]string{}
	readJson(fname, func(k string, v interface{}) {
		if k == "api_data" {
			for _, x := range v.([]interface{}) {
				item := x.(map[string]interface{})
				itemId := toInt(item["api_id"])
				id := toInt(item["api_slotitem_id"])
				name := itemTable[id]
				eq := equipTable[itemId]
				odata = append(odata, []string{strconv.Itoa(id), strconv.Itoa(itemId), name, eq})
			}
		}
	})
	sort.Sort(itemlist(odata))
	odata = append([][]string{[]string{"ID", "ITEMID", "名前", "装備艦"}}, odata...)
	err := csv.NewWriter(ofile).WriteAll(odata)
	if err != nil {
		panic(err)
	}
}

func readShip3(sname string, proc func(i int, item map[string]interface{})) {
	readJson(sname, func(k string, v interface{}) {
		if k == "api_data" {
			for k, x := range v.(map[string]interface{}) {
				if k == "api_ship" {
					for i, val := range x.([]interface{}) {
						proc(i, val.(map[string]interface{}))
					}
				}
			}
		}
	})
}

func loadEquipTable() {
	readShip3("port.json", func(i int, item map[string]interface{}) {
		kname := shipTable[toInt(item["api_ship_id"])].name
		for _, v := range item["api_slot"].([]interface{}) {
			equipTable[toInt(v)] = kname
		}
	})
	log.Println("load 'port.json'")
}

func toHhmmss(ti int) (int, int, int) {
	hh := ti / 3600000
	mm := (ti - hh*3600000) / 60000
	ss := (ti - hh*3600000 - mm*60000) / 1000
	return hh, mm, ss
}
func shipData(i int, mm map[string]interface{}) []string {
	odata := []string{}
	sid := toInt(mm["api_ship_id"])
	name := shipTable[sid].name
	stype := shipTable[sid].stype
	sname := typeTable[stype]
	odata = append(odata, "", "", name, sname)
	for _, h := range dispItems {
		if h == "api_ndock_time" {
			ti := toInt(mm[h])
			if ti > 0 {
				hh, mm, ss := toHhmmss(ti)
				odata = append(odata, fmt.Sprintf("%02d:%02d:%02d", hh, mm, ss))
			} else {
				odata = append(odata, "")
			}
		} else if h == "api_exp" {
			v := mm[h].([]interface{})
			odata = append(odata, fmt.Sprint(v[1]))
		} else if h == "api_ndock_item" {
			v := mm[h].([]interface{})
			if toInt(v[0]) > 0 || toInt(v[1]) > 0 {
				odata = append(odata, fmt.Sprint(mm[h]))
			} else {
				odata = append(odata, "")
			}
		} else {
			odata = append(odata, fmt.Sprint(mm[h]))
		}
	}
	odata = append(odata, fmt.Sprintf("%d", stype))
	return odata
}

type sorting [][]string

func (s sorting) Len() int {
	return len(s)
}

func (s sorting) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func atoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}
func (s sorting) Less(i, j int) bool {
	if atoi(s[i][9]) < atoi(s[j][9]) {
		return true
	}
	if atoi(s[i][9]) > atoi(s[j][9]) {
		return false
	}
	return atoi(s[i][4]) > atoi(s[j][4])
}

func ship3List(ofile io.Writer, fname string) {
	odata := [][]string{}
	readShip3(fname, func(i int, item map[string]interface{}) {
		odata = append(odata, shipData(i, item))
	})
	sort.Sort(sorting(odata))
	for i, _ := range odata {
		odata[i][0] = fmt.Sprintf(ship3Exp1, i+2)
		odata[i][1] = fmt.Sprintf(ship3Exp2, i+2)
	}
	odata = append([][]string{[]string{"遠征回数", "疲労", "名", "艦種", "レベル", "状態", "EXP", "修理資源", "修理時間", "艦種id"}}, odata...)
	err := csv.NewWriter(ofile).WriteAll(odata)
	if err != nil {
		panic(err)
	}
}

func strip(body []byte) []byte {
	i := bytes.Index(body, []byte("{"))
	j := bytes.LastIndex(body, []byte("}"))
	return body[i:(j + 1)]
}
func unmarshal(fname string) map[string]interface{} {
	js, err := ioutil.ReadFile(fname)
	if err != nil {
		exit(err)
	}
	jp := make(map[string]interface{})
	err = json.Unmarshal(strip(js), &jp)
	if err != nil {
		exit(err)
	}
	return jp
}

func sortedIntKeys(mm interface{}) []int {
	var keys []int
	switch m := mm.(type) {
	case map[int]string:
		for k, _ := range m {
			keys = append(keys, k)
		}
	case map[int]ship:
		for k, _ := range m {
			keys = append(keys, k)
		}
	default:
		panic("coding error")
	}
	sort.Ints(keys)
	return keys
}

func printer(oname string, proc func(ofile io.Writer)) {
	ofile, err := os.Create(oname)
	if err != nil {
		exit(err)
	}
	defer func() {
		err = ofile.Close()
		if err != nil {
			exit(err)
		}
	}()
	proc(ofile)
}

func printTable(fname string, table map[int]string) {
	printer(fname, func(ofile io.Writer) {
		for _, k := range sortedIntKeys(table) {
			fmt.Fprintf(ofile, "%6d,%s\n", k, table[k])
		}
	})

}

func printMaster() {
	printTable("type_master.txt", typeTable)
	printer("ship_master.txt", func(ofile io.Writer) {
		for _, k := range sortedIntKeys(shipTable) {
			fmt.Fprintf(ofile, "%6d,%-10s,%s\n", k, typeTable[shipTable[k].stype], shipTable[k].name)
		}
	})
	printTable("item_master.txt", itemTable)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
kctool [<flags>] <json file name> <output file name>`)
	fmt.Fprintln(os.Stderr, "flags:")
	flag.PrintDefaults()
	os.Exit(1)

}
func main() {
	flag.Parse()
	if load == "master" {
		loadTypeTable()
		loadShipTable()
		loadItemTable()
		printMaster()
		return
	}
	if len(flag.Args()) != 2 {
		printUsage()
	}
	ofile, err := os.Create(flag.Args()[1])
	if err != nil {
		exit(err)
	}
	defer func() {
		err = ofile.Close()
		if err != nil {
			exit(err)
		}
	}()
	fname := flag.Args()[0]
	if !dump {
		loadTypeTable()
		loadShipTable()
		if load == "slotitem" {
			loadItemTable()
			loadEquipTable()
			slotItem(ofile, fname)
			return
		} else if load == "port" {
			ship3List(ofile, fname)
			return
		}
	}
	jp := unmarshal(fname)
	parse(ofile, jp, "home", 0)
}
