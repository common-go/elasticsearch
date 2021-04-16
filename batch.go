package elasticsearch

import (
	"context"
	"errors"
	"github.com/elastic/go-elasticsearch"
	"github.com/elastic/go-elasticsearch/esutil"
	"reflect"
)

func InsertMany(ctx context.Context, es *elasticsearch.Client, indexName string, modelType reflect.Type, model interface{}) ([]int64, []int64, error) {
	value := reflect.Indirect(reflect.ValueOf(model))
	var failureIndex, successIndices, failureIndices []int64
	if value.Kind() == reflect.Slice && value.Len() > 0 {
		bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
			Index:  indexName,
			Client: es,
		})
		if err != nil {
			return successIndices, failureIndices, err
		}
		listIds := FindListIdField(modelType, model)
		var successIds, failIds []interface{}
		for i := 0; i < value.Len(); i++ {
			sliceValue := value.Index(i).Interface()
			if idIndex, _, _ := FindIdField(modelType); idIndex >= 0 {
				modelValue := reflect.Indirect(reflect.ValueOf(sliceValue))
				idValue := modelValue.Field(idIndex).String()
				if idValue != "" {
					body := BuildQueryWithoutIdFromObject(sliceValue)
					er1 := bi.Add(context.Background(), esutil.BulkIndexerItem{
						Action:     "create",
						DocumentID: idValue,
						Body:       esutil.NewJSONReader(body),
						OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {
							successIds = append(successIds, res.DocumentID)
						},
						OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
							failIds = append(failIds, res.DocumentID)
						},
					})
					if er1 != nil {
						failureIndex = append(failureIndex, int64(i))
					}
				} else {
					failureIndex = append(failureIndex, int64(i))
				}
			} else {
				failureIndex = append(failureIndex, int64(i))
			}
		}
		if er2 := bi.Close(context.Background()); er2 != nil {
			return successIndices, failureIndices, er2
		}
		successIndices, failureIndices = BuildIndicesResult(listIds, successIds, failIds)
		failureIndices = append(failureIndices, failureIndex...)
		return successIndices, failureIndices, nil
	}
	return successIndices, failureIndices, errors.New("invalid input")
}


func UpsertMany(ctx context.Context, es *elasticsearch.Client, indexName string, modelType reflect.Type, model interface{}) ([]int64, []int64, error) {
	value := reflect.Indirect(reflect.ValueOf(model))
	var failureIndex, successIndices, failureIndices []int64
	if value.Kind() == reflect.Slice && value.Len() > 0 {
		bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
			Index:  indexName,
			Client: es,
		})
		if err != nil {
			return successIndices, failureIndices, err
		}
		listIds := FindListIdField(modelType, model)
		var successIds, failIds []interface{}
		for i := 0; i < value.Len(); i++ {
			sliceValue := value.Index(i).Interface()
			if idIndex, _, _ := FindIdField(modelType); idIndex >= 0 {
				modelValue := reflect.Indirect(reflect.ValueOf(sliceValue))
				idValue := modelValue.Field(idIndex).String()
				if idValue != "" {
					body := BuildQueryWithoutIdFromObject(sliceValue)
					er1 := bi.Add(context.Background(), esutil.BulkIndexerItem{
						Action:     "index",
						DocumentID: idValue,
						Body:       esutil.NewJSONReader(body),
						OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {
							successIds = append(successIds, res.DocumentID)
						},
						OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
							failIds = append(failIds, res.DocumentID)
						},
					})
					if er1 != nil {
						failureIndex = append(failureIndex, int64(i))
					}
				} else {
					failureIndex = append(failureIndex, int64(i))
				}
			} else {
				failureIndex = append(failureIndex, int64(i))
			}
		}
		if er2 := bi.Close(context.Background()); er2 != nil {
			return successIndices, failureIndices, er2
		}
		successIndices, failureIndices = BuildIndicesResult(listIds, successIds, failIds)
		failureIndices = append(failureIndices, failureIndex...)
		return successIndices, failureIndices, nil
	}
	return successIndices, failureIndices, errors.New("invalid input")
}
