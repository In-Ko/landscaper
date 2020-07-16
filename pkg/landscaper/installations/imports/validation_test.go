// Copyright 2020 Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package imports_test

import (
	"context"
	"sync"

	"github.com/go-logr/logr/testing"
	g "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	lsv1alpha1 "github.com/gardener/landscaper/pkg/apis/core/v1alpha1"
	"github.com/gardener/landscaper/pkg/kubernetes"
	"github.com/gardener/landscaper/pkg/landscaper/datatype"
	"github.com/gardener/landscaper/pkg/landscaper/installations"
	"github.com/gardener/landscaper/pkg/landscaper/installations/imports"
	lsoperation "github.com/gardener/landscaper/pkg/landscaper/operation"
	"github.com/gardener/landscaper/pkg/landscaper/registry/fake"
	"github.com/gardener/landscaper/test/utils/fake_client"
)

var _ = g.Describe("Validation", func() {

	var (
		op *installations.Operation

		fakeInstallations map[string]*lsv1alpha1.Installation
		fakeDataTypes     map[string]*lsv1alpha1.DataType
		fakeClient        client.Client
		fakeRegistry      *fake.FakeRegistry

		once sync.Once
	)

	g.BeforeEach(func() {
		once.Do(func() {
			var (
				err   error
				state *fake_client.State
			)
			fakeClient, state, err = fake_client.NewFakeClientFromPath("./testdata/state")
			Expect(err).ToNot(HaveOccurred())

			fakeInstallations = state.Installations
			fakeDataTypes = state.DataTypes

			fakeRegistry, err = fake.NewFakeRegistryFromPath("./testdata/registry")
			Expect(err).ToNot(HaveOccurred())
		})

		dtArr := make([]lsv1alpha1.DataType, 0)
		for _, dt := range fakeDataTypes {
			dtArr = append(dtArr, *dt)
		}
		internalDataTypes, err := datatype.CreateDatatypesMap(dtArr)
		Expect(err).ToNot(HaveOccurred())

		op = &installations.Operation{
			Interface: lsoperation.NewOperation(testing.NullLogger{}, fakeClient, kubernetes.LandscaperScheme, fakeRegistry),
			Datatypes: internalDataTypes,
		}
	})

	g.Context("root", func() {
		g.It("should import data from the static config", func() {
			inInstRoot, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test1/root"])
			Expect(err).ToNot(HaveOccurred())
			op.Inst = inInstRoot

			value, err := yaml.Marshal(map[string]interface{}{
				"ext": map[string]interface{}{
					"a": "val1",
				},
			})
			Expect(err).ToNot(HaveOccurred())
			inInstRoot.Info.Spec.StaticData = []lsv1alpha1.StaticDataSource{{Value: value}}

			val := imports.NewValidator(op, nil)
			err = val.Validate(context.TODO(), inInstRoot)
			Expect(err).ToNot(HaveOccurred())
		})

		g.It("should reject the import from static data if the import is of the wrong type", func() {
			inInstRoot, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test1/root"])
			Expect(err).ToNot(HaveOccurred())
			op.Inst = inInstRoot

			value, err := yaml.Marshal(map[string]interface{}{
				"ext": map[string]interface{}{
					"a": true,
				},
			})
			Expect(err).ToNot(HaveOccurred())
			inInstRoot.Info.Spec.StaticData = []lsv1alpha1.StaticDataSource{{Value: value}}

			val := imports.NewValidator(op, nil)
			err = val.Validate(context.TODO(), inInstRoot)
			Expect(err).To(HaveOccurred())
		})
	})

	g.It("should successfully validate when the import of a component is defined by its parent with the right version", func() {
		inInstA, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test1/a"])
		Expect(err).ToNot(HaveOccurred())
		op.Inst = inInstA

		inInstRoot, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test1/root"])
		Expect(err).ToNot(HaveOccurred())

		val := imports.NewValidator(op, inInstRoot)
		err = val.Validate(context.TODO(), inInstA)
		Expect(err).ToNot(HaveOccurred())
	})

	//g.It("should reject when the import of a component is defined by its parent with a already reconciled version", func() {
	//	inInstA, err := installations.CreateInternalInstallation(fakeRegistry, fakeInstallations["test1/a"])
	//	Expect(err).ToNot(HaveOccurred())
	//
	//	instRoot := fakeInstallations["test1/root"]
	//	instRoot.Status.Imports[0].ConfigGeneration = 5
	//	inInstRoot, err := installations.CreateInternalInstallation(fakeRegistry, instRoot)
	//	Expect(err).ToNot(HaveOccurred())
	//
	//	val := imports.NewValidator(op, nil, inInstRoot)
	//	err = val.Validate(inInstA)
	//	Expect(err).To(HaveOccurred())
	//	Expect(imports.IsImportNotSatisfiedError(err)).To(BeTrue())
	//})

	g.It("should reject the validation when the parent component is not progressing", func() {
		inInstA, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test1/a"])
		Expect(err).ToNot(HaveOccurred())
		op.Inst = inInstA

		inInstRoot, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test1/root"])
		Expect(err).ToNot(HaveOccurred())
		inInstRoot.Info.Status.Phase = lsv1alpha1.ComponentPhaseInit

		val := imports.NewValidator(op, inInstRoot)
		err = val.Validate(context.TODO(), inInstA)
		Expect(err).To(HaveOccurred())
		Expect(installations.IsImportNotSatisfiedError(err)).To(BeTrue())
	})

	g.It("should reject when the import of a component is not yet ready", func() {
		inInstA, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test1/a"])
		Expect(err).ToNot(HaveOccurred())

		inInstB, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test1/b"])
		Expect(err).ToNot(HaveOccurred())

		inInstC, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test1/c"])
		Expect(err).ToNot(HaveOccurred())

		inInstD, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test1/d"])
		Expect(err).ToNot(HaveOccurred())
		op.Inst = inInstD

		inInstRoot, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test1/root"])
		Expect(err).ToNot(HaveOccurred())

		val := imports.NewValidator(op, inInstRoot, inInstA, inInstB, inInstC)
		err = val.Validate(context.TODO(), inInstD)
		Expect(err).To(HaveOccurred())
		Expect(installations.IsImportNotSatisfiedError(err)).To(BeTrue())
	})

	// that one fucked up scenario
	g.It("should reject when there is already a higher config generation in the imported tree than the config generation that should be imported", func() {
		inInstA, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test3/a"])
		Expect(err).ToNot(HaveOccurred())

		inInstB, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test3/b"])
		Expect(err).ToNot(HaveOccurred())

		inInstC, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test3/c"])
		Expect(err).ToNot(HaveOccurred())
		op.Inst = inInstC

		inInstRoot, err := installations.CreateInternalInstallation(context.TODO(), fakeRegistry, fakeInstallations["test3/root"])
		Expect(err).ToNot(HaveOccurred())

		val := imports.NewValidator(op, inInstRoot, inInstA, inInstB)
		err = val.Validate(context.TODO(), inInstC)
		Expect(err).To(HaveOccurred())
		Expect(installations.IsImportNotSatisfiedError(err)).To(BeTrue())
	})

})
