/*
Copyright 2023 Huawei Cloud Computing Technologies Co., Ltd.

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
package executor_test

import (
	"testing"

	"github.com/openGemini/openGemini/engine/executor"
	"github.com/openGemini/openGemini/engine/hybridqp"
	"github.com/openGemini/openGemini/open_src/influx/influxql"
)

func TestHashAggResultGetTime(t *testing.T) {
	rowDataType := buildHashAggInputRowDataType1()
	outRowDataType := buildHashAggOutputRowDataType1()

	aggFuncsName := buildAggFuncsName()
	exprOpt := make([]hybridqp.ExprOptions, len(outRowDataType.Fields()))
	for i := range outRowDataType.Fields() {
		if aggFuncsName[i] == "percentile" {
			exprOpt[i] = hybridqp.ExprOptions{
				Expr: &influxql.Call{Name: "percentile", Args: []influxql.Expr{hybridqp.MustParseExpr(rowDataType.Field(i).Expr.(*influxql.VarRef).Val), hybridqp.MustParseExpr("50")}},
				Ref:  influxql.VarRef{Val: outRowDataType.Field(i).Expr.(*influxql.VarRef).Val, Type: outRowDataType.Field(i).Expr.(*influxql.VarRef).Type},
			}
		} else {
			exprOpt[i] = hybridqp.ExprOptions{
				Expr: &influxql.Call{Name: aggFuncsName[i], Args: []influxql.Expr{hybridqp.MustParseExpr(rowDataType.Field(i).Expr.(*influxql.VarRef).Val)}},
				Ref:  influxql.VarRef{Val: outRowDataType.Field(i).Expr.(*influxql.VarRef).Val, Type: outRowDataType.Field(i).Expr.(*influxql.VarRef).Type},
			}
		}
	}
	trans := executor.HashAggTransform{}
	trans.InitFuncs(rowDataType, outRowDataType, exprOpt)
	for _, fn := range trans.GetFuncs() {
		fn.NewAggOperator().GetTime()
	}
}
