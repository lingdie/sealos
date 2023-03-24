// Copyright © 2023 sealos.
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

package crdb

import (
	"context"
	"fmt"
	"reflect"

	"github.com/labring/crdbase/query"
	"github.com/labring/crdbase/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type ModelAction struct {
	CRDBase
	ModelSchema

	gvk schema.GroupVersionKind
}

func (crdb *CRDBase) Model(m Model) *ModelAction {
	modelSchema := GetCRDModelSchema(m)

	return &ModelAction{
		CRDBase:     *crdb,
		ModelSchema: *modelSchema,

		gvk: schema.GroupVersionKind{
			Group:   crdb.GroupVersion.Group,
			Version: crdb.GroupVersion.Version,
			Kind:    modelSchema.Kind(),
		},
	}
}

// MutateFn is a function which mutates the existing object into its desired state.
type MutateFn func() error

// mutate wraps a MutateFn and applies validation to its result.
func mutate(f MutateFn, key client.ObjectKey, obj client.Object) error {
	if err := f(); err != nil {
		return err
	}
	if newKey := client.ObjectKeyFromObject(obj); key != newKey {
		return fmt.Errorf("MutateFn cannot mutate object name and/or object namespace")
	}
	return nil
}

func (ma *ModelAction) NamespacedName(name string) types.NamespacedName {
	return types.NamespacedName{Namespace: ma.Namespace, Name: name}
}

func (ma *ModelAction) NewUnstructured() *unstructured.Unstructured {
	un := &unstructured.Unstructured{}
	un.SetGroupVersionKind(ma.gvk)
	return un
}

func (ma *ModelAction) NewUnstructuredList() *unstructured.UnstructuredList {
	unl := &unstructured.UnstructuredList{}
	unl.SetGroupVersionKind(ma.gvk)
	return unl
}

func (ma *ModelAction) Create(ctx context.Context, data Data) (string, controllerutil.OperationResult, error) {
	// Unstructured to create CR
	cr, err := ma.Data2Unstructured(data)
	if err != nil {
		return "", controllerutil.OperationResultNone, err
	}
	if err := ma.client.Create(ctx, cr); err != nil {
		return "", controllerutil.OperationResultNone, err
	}
	return cr.GetName(), controllerutil.OperationResultCreated, nil
}

// Update updates the object with the given mutate function.
func (ma *ModelAction) Update(ctx context.Context, data Data) (string, controllerutil.OperationResult, error) {
	if reflect.TypeOf(data).Kind() == reflect.Slice {
		return "", controllerutil.OperationResultNone, fmt.Errorf("data must be a pointer to a struct, not slice")
	}
	// Unstructured to create CR
	obj, err := ma.Data2Unstructured(data)
	if err != nil {
		return "", controllerutil.OperationResultNone, err
	}

	name, optRes, err := utils.UpdateWithRetry(ctx, ma.client, obj)
	if err != nil {
		return name, optRes, err
	}
	return name, optRes, nil
}

// UpdateWithMutator updates the object with the given mutate function, object must exist.
func (ma *ModelAction) UpdateWithMutator(ctx context.Context, data Data, f MutateFn) (string, controllerutil.OperationResult, error) {
	if reflect.TypeOf(data).Kind() == reflect.Slice {
		return "", controllerutil.OperationResultNone, fmt.Errorf("data must be a pointer to a struct, not slice")
	}

	// Unstructured to create CR
	obj, err := ma.Data2Unstructured(data)
	if err != nil {
		return "", controllerutil.OperationResultNone, err
	}

	name, optRes, err := utils.CreateOrUpdateWithRetry(ctx, ma.client, obj, func() error {
		// get the latest version of the object
		ma.client.Get(ctx, ma.NamespacedName(obj.GetName()), obj)
		ma.Unstructured2Data(obj, data)
		// mutate the object
		if err = f(); err != nil {
			return err
		}
		obj, _ = ma.Data2Unstructured(data)
		return nil
	})
	if err != nil {
		return name, optRes, err
	}
	ma.Unstructured2Data(obj, data)
	return name, optRes, nil
}

func (ma *ModelAction) CreateOrUpdateList(ctx context.Context, model any, f MutateFn) (string, controllerutil.OperationResult, error) {
	if reflect.TypeOf(model).Kind() != reflect.Slice {
		return "", controllerutil.OperationResultNone, fmt.Errorf("model must be a pointer to a struct, not slice")
	}
	// TODO impl. add handle logic and test

	return "", controllerutil.OperationResultNone, nil
}

// Delete deletes the given object by name from datastore.
func (ma *ModelAction) Delete(ctx context.Context, name string) error {
	// TODO add test
	deleteObj := ma.NewUnstructured()
	deleteObj.SetNamespace(ma.Namespace)
	deleteObj.SetName(name)

	return ma.client.Delete(ctx, deleteObj)
}

func (ma *ModelAction) DeleteAllOf(ctx context.Context, query query.Query) error {
	// TODO impl. add handle logic and test
	return nil
}

func (ma *ModelAction) Get(ctx context.Context, q query.Query, data Data) error {
	// use query to get list options
	opts := q.GenListOptions()

	// get all objects using list
	dirty := ma.NewUnstructuredList()
	if err := ma.client.List(ctx, dirty, opts...); err != nil {
		return err
	}

	// do query
	res, err := ma.doQuery(dirty, q)
	if err != nil {
		return err
	}

	// if data is not a list, then return the first item
	if !utils.EnsureStructSlice(data) {
		if res.Items == nil || len(res.Items) == 0 {
			return fmt.Errorf("no result found")
		}
		if err := ma.Unstructured2Data(&res.Items[0], data); err != nil {
			return fmt.Errorf("failed to convert map to struct: %w", err)
		}
	} else {
		if err := ma.UnstructuredList2DataList(res, data); err != nil {
			return fmt.Errorf("failed to convert map to struct: %w", err)
		}
	}
	return nil
}

func (ma *ModelAction) Data2Unstructured(data Data) (*unstructured.Unstructured, error) {
	name := ma.GetPrimaryFieldValue(data)
	if name == "" && ma.PrimaryField != "" {
		return nil, fmt.Errorf("Data2Unstructured error: primary field %s has been setted, but value is empty", ma.PrimaryField)
	} else if name == "" {
		name = utils.GenerateMetaName()
	}

	modelMap, err := utils.StructJSON2Map(data)
	if err != nil {
		return nil, fmt.Errorf("Data2Unstructured error: convert model to Unstructured fail: %w", err)
	}

	// Unstructured For create CR
	mcr := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": ma.ApiVersion(),
			"kind":       ma.Names.Kind,
			"metadata": map[string]any{
				"name":      name,
				"namespace": ma.Namespace,
				//"labels": map[string]any{
				//	crdBaseURL + "/managed-by": providerName,
				//},
			},
			"spec": modelMap,
		},
	}

	y, _ := yaml.Marshal(mcr)
	ma.log.V(1).Info("Data2Unstructured", "unstructured", string(y))

	return mcr, nil
}

// Unstructured2Data draft impl. TODO: review and test this!
func (ma *ModelAction) Unstructured2Data(u *unstructured.Unstructured, data Data) error {
	if err := utils.Map2JSONStruct(u.UnstructuredContent()["spec"].(map[string]any), &data); err != nil {
		return fmt.Errorf("failed to convert map to struct: %w", err)
	}
	return nil
}

// UnstructuredList2DataList draft impl. TODO: review and test this!
func (ma *ModelAction) UnstructuredList2DataList(ul *unstructured.UnstructuredList, datas Data) error {
	for _, u := range ul.Items {
		var data Data
		if err := utils.Map2JSONStruct(u.UnstructuredContent()["spec"].(map[string]any), &data); err != nil {
			return fmt.Errorf("failed to convert map to struct: %w", err)
		}
		reflect.ValueOf(datas).Elem().Set(reflect.Append(reflect.ValueOf(datas).Elem(), reflect.ValueOf(data)))
	}
	return nil
}

func (ma *ModelAction) doQuery(in *unstructured.UnstructuredList, q query.Query) (*unstructured.UnstructuredList, error) {
	res := in.DeepCopy()
	pipeline := ma.getDoQueryPipeLine()
	for _, f := range pipeline {
		var err error
		res, err = f(res, q)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (ma *ModelAction) getDoQueryPipeLine() []func(list *unstructured.UnstructuredList, q query.Query) (*unstructured.UnstructuredList, error) {
	return []func(list *unstructured.UnstructuredList, q query.Query) (*unstructured.UnstructuredList, error){
		ma.doFilter,
		ma.doSort,
		ma.doDistinct,
		ma.doPagination,
	}
}

func (ma *ModelAction) doFilter(in *unstructured.UnstructuredList, q query.Query) (*unstructured.UnstructuredList, error) {
	res := ma.NewUnstructuredList()
	for _, item := range in.Items {
		// TODO do filters in q.filter
		isMatch := true
		for _, f := range q.Filter {
			content, err := utils.GetValueFormUnstructuredContent(item.UnstructuredContent(), fmt.Sprintf("spec.%s", f.Field))
			if err != nil {
				return ma.NewUnstructuredList(), err
			}
			switch f.Operator {
			case selection.Equals:
				// TODO impl.
				if content != f.Value {
					isMatch = false
				}
			}
		}
		if isMatch {
			res.Items = append(res.Items, item)
		}
	}
	return res, nil
}

func (ma *ModelAction) doSort(in *unstructured.UnstructuredList, q query.Query) (*unstructured.UnstructuredList, error) {
	// TODO impl.
	return in, nil
}

func (ma *ModelAction) doDistinct(in *unstructured.UnstructuredList, q query.Query) (*unstructured.UnstructuredList, error) {
	// TODO impl.
	return in, nil
}

func (ma *ModelAction) doPagination(in *unstructured.UnstructuredList, q query.Query) (*unstructured.UnstructuredList, error) {
	// TODO impl.
	return in, nil
}
