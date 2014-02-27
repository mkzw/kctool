package main

// kctool
// master file name (need)
// stype.json
// ship.json
//
// load type name
// ship3
// slotitem (need ship3.json)

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
	ship3Exp1 = `=IF(F%[1]d>49,ROUNDUP((F%[1]d-49)/3),"")`
	ship3Exp2 = `=IF(F%[1]d<49,49-F%[1]d,"")`
)

var (
	dump       bool
	load       string
	typeTable  = make(map[int]string)
	shipTable  = make(map[int]ship)
	equipTable = make(map[int]string)
	dispItems  = []string{"api_lv", "api_cond", "api_exp", "api_ndock_item", "api_ndock_time"}
)

type ship struct {
	stype int
	name  string
}

func init() {
	flag.BoolVar(&dump, "dump", false, "dump row data")
	flag.StringVar(&load, "load", "ship3", "load data type")
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

func loadTable(sname string, proc func(item map[string]interface{})) {
	readJson(sname, func(k string, v interface{}) {
		if k == "api_data" {
			for _, x := range v.([]interface{}) {
				proc(x.(map[string]interface{}))
			}
		}
	})
}

func loadTypeTable() {
	loadTable("stype.json", func(item map[string]interface{}) {
		typeTable[toInt(item["api_id"])] = item["api_name"].(string)
	})
	log.Println("load 'stype.json'")
}

func loadShipTable() {
	loadTable("ship.json", func(item map[string]interface{}) {
		n := item["api_name"].(string)
		if n == "なし" {
			return
		}
		val := ship{stype: toInt(item["api_stype"]), name: n}
		shipTable[toInt(item["api_id"])] = val
	})
	log.Println("load 'ship.json'")
}

func slotItem(ofile io.Writer, fname string) {
	output(ofile, "ID,ITEMID,名前,装備艦\n")
	loadTable(fname, func(item map[string]interface{}) {
		id := toInt(item["api_id"])
		itemId := toInt(item["api_slotitem_id"])
		name := item["api_name"].(string)
		eq := equipTable[id]
		output(ofile, "%d,%d,%s,%s\n", id, itemId, name, eq)
	})
}
func readShip3(sname string, proc func(i int, item map[string]interface{})) {
	readJson(sname, func(k string, v interface{}) {
		if k == "api_data" {
			for k, x := range v.(map[string]interface{}) {
				if k == "api_ship_data" {
					for i, val := range x.([]interface{}) {
						proc(i, val.(map[string]interface{}))
					}
				}
			}
		}
	})
}

func loadEquipTable() {
	readShip3("ship3.json", func(i int, item map[string]interface{}) {
		kname := shipTable[toInt(item["api_ship_id"])].name
		for _, v := range item["api_slot"].([]interface{}) {
			equipTable[toInt(v)] = kname
		}
	})
	log.Println("load 'ship3.json'")
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
	for i,_ := range odata {
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

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
kctool [<flags>] <json file name> <output file name>`)
	fmt.Fprintln(os.Stderr, "flags:")
	flag.PrintDefaults()
	os.Exit(1)

}
func main() {
	flag.Parse()
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
			loadEquipTable()
			slotItem(ofile, fname)
			return
		} else if load == "ship3" {
			ship3List(ofile, fname)
			return
		}
	}
	jp := unmarshal(fname)
	parse(ofile, jp, "home", 0)
}
