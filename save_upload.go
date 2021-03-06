// Copyright 2017 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbtester

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/dbtester/dbtesterpb"
	"github.com/coreos/dbtester/pkg/remotestorage"
	"github.com/coreos/dbtester/pkg/report"
	humanize "github.com/dustin/go-humanize"
	"github.com/gyuho/dataframe"
)

// DatasizeOnDiskSummaryColumns defines summary columns.
var DatasizeOnDiskSummaryColumns = []string{
	"INDEX",
	"DATABASE-ENDPOINT",
	"TOTAL-DATA-SIZE",
	"TOTAL-DATA-SIZE-BYTES-NUM",
}

// SaveDatasizeOnDiskSummary saves data size summary.
func (cfg *Config) SaveDatasizeOnDiskSummary(databaseID string, idxToResponse map[int]dbtesterpb.Response) error {
	gcfg, ok := cfg.DatabaseIDToTestGroup[databaseID]
	if !ok {
		return fmt.Errorf("%q does not exist", databaseID)
	}

	c1 := dataframe.NewColumn(DatasizeOnDiskSummaryColumns[0])
	c2 := dataframe.NewColumn(DatasizeOnDiskSummaryColumns[1])
	c3 := dataframe.NewColumn(DatasizeOnDiskSummaryColumns[2])
	c4 := dataframe.NewColumn(DatasizeOnDiskSummaryColumns[3])
	for i := range gcfg.DatabaseEndpoints {
		c1.PushBack(dataframe.NewStringValue(i))
		c2.PushBack(dataframe.NewStringValue(gcfg.DatabaseEndpoints[i]))
		c3.PushBack(dataframe.NewStringValue(humanize.Bytes(uint64(idxToResponse[i].DatasizeOnDisk))))
		c4.PushBack(dataframe.NewStringValue(idxToResponse[i].DatasizeOnDisk))
	}

	fr := dataframe.New()
	if err := fr.AddColumn(c1); err != nil {
		return err
	}
	if err := fr.AddColumn(c2); err != nil {
		return err
	}
	if err := fr.AddColumn(c3); err != nil {
		return err
	}
	if err := fr.AddColumn(c4); err != nil {
		return err
	}

	return fr.CSV(cfg.Control.ServerDatasizeOnDiskSummaryPath)
}

func (cfg *Config) saveDataLatencyDistributionSummary(st report.Stats) {
	fr := dataframe.New()

	c1 := dataframe.NewColumn("TOTAL-SECONDS")
	c1.PushBack(dataframe.NewStringValue(fmt.Sprintf("%4.4f", st.Total.Seconds())))
	if err := fr.AddColumn(c1); err != nil {
		plog.Fatal(err)
	}

	c2 := dataframe.NewColumn("REQUESTS-PER-SECOND")
	c2.PushBack(dataframe.NewStringValue(fmt.Sprintf("%4.4f", st.RPS)))
	if err := fr.AddColumn(c2); err != nil {
		plog.Fatal(err)
	}

	c3 := dataframe.NewColumn("SLOWEST-LATENCY-MS")
	c3.PushBack(dataframe.NewStringValue(fmt.Sprintf("%4.4f", 1000*st.Slowest)))
	if err := fr.AddColumn(c3); err != nil {
		plog.Fatal(err)
	}

	c4 := dataframe.NewColumn("FASTEST-LATENCY-MS")
	c4.PushBack(dataframe.NewStringValue(fmt.Sprintf("%4.4f", 1000*st.Fastest)))
	if err := fr.AddColumn(c4); err != nil {
		plog.Fatal(err)
	}

	c5 := dataframe.NewColumn("AVERAGE-LATENCY-MS")
	c5.PushBack(dataframe.NewStringValue(fmt.Sprintf("%4.4f", 1000*st.Average)))
	if err := fr.AddColumn(c5); err != nil {
		plog.Fatal(err)
	}

	c6 := dataframe.NewColumn("STDDEV-LATENCY-MS")
	c6.PushBack(dataframe.NewStringValue(fmt.Sprintf("%4.4f", 1000*st.Stddev)))
	if err := fr.AddColumn(c6); err != nil {
		plog.Fatal(err)
	}

	if len(st.ErrorDist) > 0 {
		for errName, errN := range st.ErrorDist {
			errcol := dataframe.NewColumn(fmt.Sprintf("ERROR: %q", errName))
			errcol.PushBack(dataframe.NewStringValue(errN))
			if err := fr.AddColumn(errcol); err != nil {
				plog.Fatal(err)
			}
		}
	} else {
		errcol := dataframe.NewColumn("ERROR")
		errcol.PushBack(dataframe.NewStringValue("0"))
		if err := fr.AddColumn(errcol); err != nil {
			plog.Fatal(err)
		}
	}

	if err := fr.CSVHorizontal(cfg.Control.ClientLatencyDistributionSummaryPath); err != nil {
		plog.Fatal(err)
	}
}

func (cfg *Config) saveDataLatencyDistributionPercentile(st report.Stats) {
	pctls, seconds := report.Percentiles(st.Lats)
	c1 := dataframe.NewColumn("LATENCY-PERCENTILE")
	c2 := dataframe.NewColumn("LATENCY-MS")
	for i := range pctls {
		pct := fmt.Sprintf("p%.1f", pctls[i])
		if strings.HasSuffix(pct, ".0") {
			pct = strings.Replace(pct, ".0", "", -1)
		}

		c1.PushBack(dataframe.NewStringValue(pct))
		c2.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", 1000*seconds[i])))
	}

	fr := dataframe.New()
	if err := fr.AddColumn(c1); err != nil {
		plog.Fatal(err)
	}
	if err := fr.AddColumn(c2); err != nil {
		plog.Fatal(err)
	}
	if err := fr.CSV(cfg.Control.ClientLatencyDistributionPercentilePath); err != nil {
		plog.Fatal(err)
	}
}

func (cfg *Config) saveDataLatencyDistributionAll(st report.Stats) {
	min := int64(math.MaxInt64)
	max := int64(-100000)
	rm := make(map[int64]int64)
	for _, lt := range st.Lats {
		// convert second(float64) to millisecond
		ms := lt * 1000

		// truncate all digits below 10ms
		// (e.g. 125.11ms becomes 120ms)
		v := int64(math.Trunc(ms/10) * 10)
		if _, ok := rm[v]; !ok {
			rm[v] = 1
		} else {
			rm[v]++
		}

		if min > v {
			min = v
		}
		if max < v {
			max = v
		}
	}

	c1 := dataframe.NewColumn("LATENCY-MS")
	c2 := dataframe.NewColumn("COUNT")
	cur := min
	for {
		c1.PushBack(dataframe.NewStringValue(fmt.Sprintf("%d", int64(cur))))
		v, ok := rm[cur]
		if ok {
			c2.PushBack(dataframe.NewStringValue(fmt.Sprintf("%d", v)))
		} else {
			c2.PushBack(dataframe.NewStringValue("0"))
		}
		cur += 10
		if cur-10 == max { // was last point
			break
		}
	}
	fr := dataframe.New()
	if err := fr.AddColumn(c1); err != nil {
		plog.Fatal(err)
	}
	if err := fr.AddColumn(c2); err != nil {
		plog.Fatal(err)
	}
	if err := fr.CSV(cfg.Control.ClientLatencyDistributionAllPath); err != nil {
		plog.Fatal(err)
	}
}

func (cfg *Config) saveDataLatencyThroughputTimeseries(gcfg TestGroup, st report.Stats, tsToClientN map[int64]int64) {
	c1 := dataframe.NewColumn("UNIX-SECOND")
	c2 := dataframe.NewColumn("CONTROL-CLIENT-NUM")
	c3 := dataframe.NewColumn("MIN-LATENCY-MS")
	c4 := dataframe.NewColumn("AVG-LATENCY-MS")
	c5 := dataframe.NewColumn("MAX-LATENCY-MS")
	c6 := dataframe.NewColumn("AVG-THROUGHPUT")
	for i := range st.TimeSeries {
		// this Timestamp is unix seconds
		c1.PushBack(dataframe.NewStringValue(fmt.Sprintf("%d", st.TimeSeries[i].Timestamp)))

		if len(tsToClientN) == 0 {
			c2.PushBack(dataframe.NewStringValue(fmt.Sprintf("%d", gcfg.ClientNumber)))
		} else {
			c2.PushBack(dataframe.NewStringValue(fmt.Sprintf("%d", tsToClientN[st.TimeSeries[i].Timestamp])))
		}

		c3.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", toMillisecond(st.TimeSeries[i].MinLatency))))
		c4.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", toMillisecond(st.TimeSeries[i].AvgLatency))))
		c5.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", toMillisecond(st.TimeSeries[i].MaxLatency))))

		c6.PushBack(dataframe.NewStringValue(fmt.Sprintf("%d", st.TimeSeries[i].ThroughPut)))
	}

	fr := dataframe.New()
	if err := fr.AddColumn(c1); err != nil {
		plog.Fatal(err)
	}
	if err := fr.AddColumn(c2); err != nil {
		plog.Fatal(err)
	}
	if err := fr.AddColumn(c3); err != nil {
		plog.Fatal(err)
	}
	if err := fr.AddColumn(c4); err != nil {
		plog.Fatal(err)
	}
	if err := fr.AddColumn(c5); err != nil {
		plog.Fatal(err)
	}
	if err := fr.AddColumn(c6); err != nil {
		plog.Fatal(err)
	}

	if err := fr.CSV(cfg.Control.ClientLatencyThroughputTimeseriesPath); err != nil {
		plog.Fatal(err)
	}

	// aggregate latency by the number of keys
	tss := processTimeSeries(st.TimeSeries, 1000, gcfg.RequestNumber)
	ctt1 := dataframe.NewColumn("KEYS")
	ctt2 := dataframe.NewColumn("MIN-LATENCY-MS")
	ctt3 := dataframe.NewColumn("AVG-LATENCY-MS")
	ctt4 := dataframe.NewColumn("MAX-LATENCY-MS")
	for i := range tss {
		ctt1.PushBack(dataframe.NewStringValue(tss[i].keyNum))
		ctt2.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", toMillisecond(tss[i].minLat))))
		ctt3.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", toMillisecond(tss[i].avgLat))))
		ctt4.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", toMillisecond(tss[i].maxLat))))
	}

	frr := dataframe.New()
	if err := frr.AddColumn(ctt1); err != nil {
		plog.Fatal(err)
	}
	if err := frr.AddColumn(ctt2); err != nil {
		plog.Fatal(err)
	}
	if err := frr.AddColumn(ctt3); err != nil {
		plog.Fatal(err)
	}
	if err := frr.AddColumn(ctt4); err != nil {
		plog.Fatal(err)
	}

	if err := frr.CSV(cfg.Control.ClientLatencyByKeyNumberPath); err != nil {
		plog.Fatal(err)
	}
}

func (cfg *Config) saveAllStats(gcfg TestGroup, stats report.Stats, tsToClientN map[int64]int64) {
	cfg.saveDataLatencyDistributionSummary(stats)
	cfg.saveDataLatencyDistributionPercentile(stats)
	cfg.saveDataLatencyDistributionAll(stats)
	cfg.saveDataLatencyThroughputTimeseries(gcfg, stats, tsToClientN)
}

// UploadToGoogle uploads target file to Google Cloud Storage.
func (cfg *Config) UploadToGoogle(databaseID string, targetPath string) error {
	gcfg, ok := cfg.DatabaseIDToTestGroup[databaseID]
	if !ok {
		return fmt.Errorf("%q does not exist", databaseID)
	}
	if !exist(targetPath) {
		return fmt.Errorf("%q does not exist", targetPath)
	}
	u, err := remotestorage.NewGoogleCloudStorage([]byte(cfg.Control.GoogleCloudStorageKey), cfg.Control.GoogleCloudProjectName)
	if err != nil {
		return err
	}

	srcPath := targetPath
	dstPath := filepath.Base(targetPath)
	if !strings.HasPrefix(dstPath, gcfg.DatabaseTag) {
		dstPath = fmt.Sprintf("%s-%s", gcfg.DatabaseTag, dstPath)
	}
	dstPath = filepath.Join(cfg.Control.GoogleCloudStorageSubDirectory, dstPath)

	var uerr error
	for k := 0; k < 30; k++ {
		if uerr = u.UploadFile(cfg.Control.GoogleCloudStorageBucketName, srcPath, dstPath); uerr != nil {
			plog.Printf("#%d: error %v while uploading %q", k, uerr, targetPath)
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	return uerr
}
