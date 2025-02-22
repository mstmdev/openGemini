/*
Copyright 2022 Huawei Cloud Computing Technologies Co., Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package coordinator

import (
	"fmt"
	"sync"
	"testing"

	"github.com/influxdata/influxdb/models"
	"github.com/openGemini/openGemini/lib/errno"
	"github.com/openGemini/openGemini/lib/logger"
	"github.com/openGemini/openGemini/lib/metaclient"
	"github.com/openGemini/openGemini/lib/netstorage"
	"github.com/openGemini/openGemini/open_src/influx/influxql"
	meta2 "github.com/openGemini/openGemini/open_src/influx/meta"
	"github.com/stretchr/testify/assert"
)

func getExpRows() models.Rows {
	return models.Rows{
		&models.Row{
			Name:    "mst",
			Tags:    nil,
			Columns: []string{"key", "value"},
			Values: [][]interface{}{
				{"author", "mao"},
				{"author", "petter"},
				{"author", "san"},
				{"author", "tai"},
				{"author", "van"},
			},
		},
		&models.Row{
			Name:    "mst_2",
			Tags:    nil,
			Columns: []string{"key", "value"},
			Values: [][]interface{}{
				{"author", "mao"},
				{"author", "tai"},
			},
		},
	}
}

func getCardinalityExpRows() models.Rows {
	return models.Rows{
		&models.Row{
			Name:    "mst",
			Tags:    nil,
			Columns: []string{"count"},
			Values:  [][]interface{}{{5}},
		},
		&models.Row{
			Name:    "mst_2",
			Tags:    nil,
			Columns: []string{"count"},
			Values:  [][]interface{}{{2}},
		},
	}
}

func TestShowTagValuesExecutor(t *testing.T) {
	e := NewShowTagValuesExecutor(logger.NewLogger(errno.ModuleUnknown),
		&mockMC{}, &mockME{}, &mockNS{})
	smt := &influxql.ShowTagValuesStatement{
		Database: "db0",
		Sources:  append(influxql.Sources{}, &influxql.Measurement{}),
	}

	rows, err := e.Execute(smt)
	assert.NoError(t, err)
	assert.Equal(t, rows, getExpRows())

	// cardinality
	e.Cardinality(influxql.Dimensions{})
	rows, err = e.Execute(smt)
	assert.NoError(t, err)
	assert.Equal(t, rows, getCardinalityExpRows())

	// no database
	smt.Database = ""
	_, err = e.Execute(smt)
	assert.EqualError(t, err, ErrDatabaseNameRequired.Error())

	// QueryTagKeys return nil
	smt.Database = "db_nil"
	rows, err = e.Execute(smt)
	assert.NoError(t, err)
	assert.Equal(t, rows, models.Rows{})

	// QueryTagKeys return error
	smt.Database = "db_error"
	_, err = e.Execute(smt)
	assert.EqualError(t, err, "mock error")
}

func TestApplyLimit(t *testing.T) {
	e := &ShowTagValuesExecutor{}
	data := netstorage.TagSets{
		{Key: "a", Value: "aaa"},
		{Key: "b", Value: "bbb"},
		{Key: "a", Value: "aaa111"},
		{Key: "a", Value: "aaa"},
		{Key: "b", Value: "bbb"},
		{Key: "b", Value: "bbb111"},
		{Key: "b", Value: "bbb"},
		{Key: "c", Value: "ccc"},
	}

	format := "limit failed. exp: len=%d, got: len=%d"

	ret := e.applyLimit(0, 10, data)
	exp := 5
	assert.Equal(t, len(ret), exp, format, exp, len(ret))

	exp = 2
	ret = e.applyLimit(0, 2, data)
	assert.Equal(t, len(ret), exp, format, exp, len(ret))

	exp = 1
	ret = e.applyLimit(2, 1, data)
	assert.Equal(t, len(ret), exp, format, exp, len(ret))
	assert.Equal(t, ret[0].Value, "bbb", "limit failed. exp: bbb, got: %s", ret[0].Value)

	exp = 1
	ret = e.applyLimit(4, 10, data)
	assert.Equal(t, len(ret), exp, format, exp, len(ret))
	assert.Equal(t, ret[0].Value, "ccc", "limit failed. exp: ccc, got: %s", ret[0].Value)

	exp = 5
	ret = e.applyLimit(-1, -1, data)
	assert.Equal(t, len(ret), exp, format, exp, len(ret))

	exp = 3
	ret = e.applyLimit(2, -1, data)
	assert.Equal(t, len(ret), exp, format, exp, len(ret))

	exp = 1
	ret = e.applyLimit(-2, 1, data)
	assert.Equal(t, len(ret), exp, format, exp, len(ret))
}

type mockMC struct {
	metaclient.MetaClient
}

func (m *mockMC) MatchMeasurements(database string, ms influxql.Measurements) (map[string]*meta2.MeasurementInfo, error) {
	mst := &meta2.MeasurementInfo{Name: "mst_0000"}
	mst_map := make(map[string]*meta2.MeasurementInfo)
	mst_map["autogen.mst_0000"] = mst
	return mst_map, nil
}

func (m *mockMC) NewDownSamplePolicy(string, string, *meta2.DownSamplePolicyInfo) error {
	return nil
}

func (m *mockMC) QueryTagKeys(database string, ms influxql.Measurements, cond influxql.Expr) (map[string]map[string]struct{}, error) {
	if database == "db_nil" {
		return nil, nil
	}

	if database == "db_error" {
		return nil, fmt.Errorf("mock error")
	}

	return map[string]map[string]struct{}{
		"mst":  {"author": struct{}{}},
		"mst2": {"author": struct{}{}},
	}, nil
}

type mockNS struct {
	netstorage.NetStorage
}

func (m *mockNS) ShowTagKeys(nodeID uint64, db string, ptId []uint32, measurements []string, condition influxql.Expr) ([]string, error) {
	if nodeID == 1 {
		arr := []string{
			"mst,tag1,tag2,tag3,tag4",
			"mst,tag2,tag4,tag6",
		}
		return arr, nil
	}
	if nodeID == 2 {
		arr := []string{
			"mst,tag1,tag2,tag3,tag4",
			"mst,tag2,tag4,tag5",
		}
		return arr, nil
	}
	if nodeID == 3 {
		arr := []string{
			"mst,tag1,tag2,tag3,tag4",
			"mst,tag2,tag4,tag6",
		}
		return arr, nil
	}
	arr := []string{
		"mst,tag1,tag2,tag3,tag4",
		"mst,tag2,tag4,tag6",
		"mst,tk1,tk2,tk3",
		"mst,tk1,tk2,tk10",
	}
	return arr, nil
}

func (m *mockNS) TagValues(nodeID uint64, db string, ptIDs []uint32, tagKeys map[string]map[string]struct{}, cond influxql.Expr) (netstorage.TablesTagSets, error) {
	if nodeID == 1 {
		return append(netstorage.TablesTagSets{}, netstorage.TableTagSets{
			Name: "mst",
			Values: netstorage.TagSets{
				{Key: "author", Value: "petter"},
				{Key: "author", Value: "van"},
				{Key: "author", Value: "san"},
			},
		}), nil
	}

	if nodeID == 2 {
		return append(netstorage.TablesTagSets{}, netstorage.TableTagSets{
			Name: "mst_2",
			Values: netstorage.TagSets{
				{Key: "author", Value: "mao"},
				{Key: "author", Value: "tai"},
			},
		}), nil
	}

	if nodeID == 3 {
		return append(netstorage.TablesTagSets{}, netstorage.TableTagSets{
			Name:   "mst_2",
			Values: netstorage.TagSets{},
		}), nil
	}

	return append(netstorage.TablesTagSets{}, netstorage.TableTagSets{
		Name: "mst",
		Values: netstorage.TagSets{
			{Key: "author", Value: "mao"},
			{Key: "author", Value: "tai"},
			{Key: "author", Value: "san"},
		},
	}), nil
}

type mockME struct {
	MetaExecutor
}

func (m *mockME) EachDBNodes(database string, fn func(nodeID uint64, pts []uint32, hasErr *bool) error) error {
	n := 4
	wg := sync.WaitGroup{}
	wg.Add(n)
	hasErr := false
	var mu sync.RWMutex
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		go func(nodeID int) {
			mu.Lock()
			errs[nodeID] = fn(uint64(nodeID), nil, &hasErr)
			mu.Unlock()
			wg.Done()
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
