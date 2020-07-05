package elasticsearch

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/elastic/go-elasticsearch"
	"github.com/elastic/go-elasticsearch/esapi"
	"github.com/elastic/go-elasticsearch/esutil"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type Config struct {
	Addresses []string `mapstructure:"addresses"`
}

func Connect(config Config) (*elasticsearch.Client, error) {
	c := elasticsearch.Config{
		Addresses: config.Addresses,
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: time.Minute,
			DialContext:           (&net.Dialer{Timeout: time.Second}).DialContext,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS11,
				// ...
			},
		},
	}
	return elasticsearch.NewClient(c)
}

func FindIdField(modelType reflect.Type) (int, string) {
	return FindFieldByJson(modelType, "_id")
}

func FindFieldByName(modelType reflect.Type, fieldName string) (index int, jsonTagName string) {
	numField := modelType.NumField()
	for index := 0; index < numField; index++ {
		field := modelType.Field(index)
		if field.Name == fieldName {
			jsonTagName := fieldName
			if jsonTag, ok := field.Tag.Lookup("json"); ok {
				jsonTagName = strings.Split(jsonTag, ",")[0]
			}
			return index, jsonTagName
		}
	}
	return -1, fieldName
}

func FindFieldByJson(modelType reflect.Type, jsonTagName string) (index int, fieldName string) {
	numField := modelType.NumField()
	for i := 0; i < numField; i++ {
		field := modelType.Field(i)
		tag1, ok1 := field.Tag.Lookup("json")
		if ok1 && strings.Split(tag1, ",")[0] == jsonTagName {
			return i, field.Name
		}
	}
	return -1, jsonTagName
}

func FindFieldByIndex(modelType reflect.Type, fieldIndex int) (fieldName, jsonTagName string) {
	if fieldIndex < modelType.NumField() {
		field := modelType.Field(fieldIndex)
		jsonTagName := ""
		if jsonTag, ok := field.Tag.Lookup("json"); ok {
			jsonTagName = strings.Split(jsonTag, ",")[0]
		}
		return field.Name, jsonTagName
	}
	return "", ""
}

func MakeMapJson(modelType reflect.Type) map[string]string {
	maps := make(map[string]string)
	numField := modelType.NumField()
	for i := 0; i < numField; i++ {
		key1 := modelType.Field(i).Name
		fields, _ := modelType.FieldByName(key1)
		if tag, ok := fields.Tag.Lookup("json"); ok {
			if strings.Contains(tag, ",") {
				a := strings.Split(tag, ",")
				maps[key1] = a[0]
			} else {
				maps[key1] = tag
			}
		} else {
			maps[key1] = key1
		}
	}
	return maps
}

//For Insert
func BuildQueryWithoutIdFromObject(object interface{}) map[string]interface{} {
	valueOf := reflect.Indirect(reflect.ValueOf(object))
	idIndex, _ := FindIdField(valueOf.Type())
	result := map[string]interface{}{}
	for i := 0; i < valueOf.NumField(); i++ {
		if i != idIndex {
			_, jsonName := FindFieldByIndex(valueOf.Type(), i)
			result[jsonName] = valueOf.Field(i).Interface()
		}
	}
	return result
}

func BuildQuery(indexName string, query map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{}
}

func MapToDBObject(object map[string]interface{}, objectMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range object {
		field := objectMap[key]
		result[field] = value
	}
	return result
}

func Exist(ctx context.Context, es *elasticsearch.Client, indexName string, documentID string) (bool, error) {
	req := esapi.ExistsRequest{
		Index:      indexName,
		DocumentID: documentID,
	}
	res, err := req.Do(ctx, es)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return false, errors.New("response error")
	} else {
		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			return false, err
		} else {
			return r["found"].(bool), nil
		}
	}
}

func FindOneById(ctx context.Context, es *elasticsearch.Client, indexName string, documentID string, modelType reflect.Type) (interface{}, error) {
	result := reflect.New(modelType).Interface()
	if ok, err := FindOneByIdAndDecode(ctx, es, indexName, documentID, result); ok {
		return result, nil
	} else {
		return nil, err
	}
}

func FindOneByIdAndDecode(ctx context.Context, es *elasticsearch.Client, indexName string, documentID string, result interface{}) (bool, error) {
	req := esapi.GetRequest{
		Index:      indexName,
		DocumentID: documentID,
	}
	res, err := req.Do(ctx, es)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return false, errors.New("response error")
	} else {
		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			return false, err
		} else {
			if err := json.NewDecoder(esutil.NewJSONReader(r["_source"])).Decode(&result); err != nil {
				return false, err
			}
			return true, nil
		}
	}
}

func FindOne(ctx context.Context, es *elasticsearch.Client, index []string, query map[string]interface{}, modelType reflect.Type) (interface{}, error) {
	result := reflect.New(modelType).Interface()
	if ok, err := FindOneAndDecode(ctx, es, index, query, result); ok {
		return result, nil
	} else {
		return nil, err
	}
}

func FindOneAndDecode(ctx context.Context, es *elasticsearch.Client, index []string, query map[string]interface{}, result interface{}) (bool, error) {
	req := esapi.SearchRequest{
		Index:          index,
		Body:           esutil.NewJSONReader(query),
		TrackTotalHits: true,
		Pretty:         true,
	}
	res, err := req.Do(ctx, es)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return false, errors.New("response error")
	} else {
		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			return false, err
		} else {
			hits := r["hits"].(map[string]interface{})["hits"].([]interface{})
			total := int(r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64))
			if total >= 1 {
				if err := json.NewDecoder(esutil.NewJSONReader(hits[0])).Decode(&result); err != nil {
					return false, err
				}
				return true, nil
			}
			return false, nil
		}
	}
}

func Find(ctx context.Context, es *elasticsearch.Client, indexName []string, query map[string]interface{}, modelType reflect.Type) (interface{}, error) {
	modelsType := reflect.Zero(reflect.SliceOf(modelType)).Type()
	result := reflect.New(modelsType).Interface()
	if ok, err := FindAndDecode(ctx, es, indexName, query, result); ok {
		return result, nil
	} else {
		return nil, err
	}
}

func FindAndDecode(ctx context.Context, es *elasticsearch.Client, indexName []string, query map[string]interface{}, result interface{}) (bool, error) {
	req := esapi.SearchRequest{
		Index:          indexName,
		Body:           esutil.NewJSONReader(query),
		TrackTotalHits: true,
		Pretty:         true,
	}
	res, err := req.Do(ctx, es)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return false, errors.New("response error")
	} else {
		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			return false, err
		} else {
			hits := r["hits"].(map[string]interface{})["hits"].([]interface{})
			if err := json.NewDecoder(esutil.NewJSONReader(hits)).Decode(&result); err != nil {
				return false, err
			}
			return true, nil
		}
	}
}

func InsertOne(ctx context.Context, es *elasticsearch.Client, indexName string, modelType reflect.Type, model interface{}) (int64, error) {
	var req esapi.CreateRequest
	if idIndex, _ := FindIdField(modelType); idIndex >= 0 {
		modelValue := reflect.Indirect(reflect.ValueOf(model))
		idValue := modelValue.Field(idIndex).String()
		body := BuildQueryWithoutIdFromObject(model)
		req = esapi.CreateRequest{
			Index:      indexName,
			DocumentID: idValue,
			Body:       esutil.NewJSONReader(body),
			Refresh:    "true",
		}
	} else {
		req = esapi.CreateRequest{
			Index:   indexName,
			Body:    esutil.NewJSONReader(model),
			Refresh: "true",
		}
	}
	res, err := req.Do(ctx, es)
	if err != nil {
		return -1, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return -1, errors.New("document ID already exists in the index")
	} else {
		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			return -1, err
		} else {
			log.Printf("[%s] %s; version=%d", res.Status(), r["result"], int(r["_version"].(float64)))
			return int64(r["_version"].(float64)), nil
		}
	}
}

func UpdateOne(ctx context.Context, es *elasticsearch.Client, indexName string, modelType reflect.Type, model interface{}) (int64, error) {
	idIndex, _ := FindIdField(modelType)
	if idIndex < 0 {
		return 0, errors.New("missing document ID in the object")
	}
	modelValue := reflect.ValueOf(model)
	idValue := modelValue.Field(idIndex).String()
	body := BuildQueryWithoutIdFromObject(model)
	req := esapi.UpdateRequest{
		Index:      indexName,
		DocumentID: idValue,
		Body:       esutil.NewJSONReader(body),
		Refresh:    "true",
	}
	res, err := req.Do(ctx, es)
	if err != nil {
		return -1, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return -1, errors.New("document ID not exists in the index")
	} else {
		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			return -1, err
		} else {
			successful := int64(r["_shards"].(map[string]interface{})["successful"].(float64))
			return successful, nil
		}
	}
}

func UpsertOne(ctx context.Context, es *elasticsearch.Client, indexName string, id string, model interface{}) (int64, error) {
	body := BuildQueryWithoutIdFromObject(model)
	req := esapi.UpdateRequest{
		Index:      indexName,
		DocumentID: id,
		Body:       esutil.NewJSONReader(body),
		Refresh:    "true",
	}
	res, err := req.Do(ctx, es)
	if err != nil {
		return -1, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return -1, errors.New("document ID not exists in the index")
	}
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return -1, err
	}
	successful := int64(r["_shards"].(map[string]interface{})["successful"].(float64))
	return successful, nil
}

func PatchOne(ctx context.Context, es *elasticsearch.Client, indexName string, model map[string]interface{}) (int64, error) {
	idValue := reflect.ValueOf(model["_id"])
	if idValue.IsZero() {
		return 0, errors.New("missing document ID in the map")
	}
	delete(model, "_id")
	req := esapi.UpdateRequest{
		Index:      indexName,
		DocumentID: idValue.String(),
		Body:       esutil.NewJSONReader(model),
		Refresh:    "true",
	}
	res, err := req.Do(ctx, es)
	if err != nil {
		return -1, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return -1, errors.New("document ID not exists in the index")
	} else {
		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			return -1, err
		} else {
			successful := int64(r["_shards"].(map[string]interface{})["successful"].(float64))
			return successful, nil
		}
	}
}

func DeleteOne(ctx context.Context, es *elasticsearch.Client, indexName string, documentID string) (int64, error) {
	req := esapi.DeleteRequest{
		Index:      indexName,
		DocumentID: documentID,
	}
	res, err := req.Do(ctx, es)
	if err != nil {
		return -1, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return -1, errors.New("document ID not exists in the index")
	} else {
		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			return -1, err
		} else {
			successful := int64(r["_shards"].(map[string]interface{})["successful"].(float64))
			return successful, nil
		}
	}
}
