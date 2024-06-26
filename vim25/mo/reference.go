/*
Copyright (c) 2015 VMware, Inc. All Rights Reserved.

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

package mo

import "github.com/zhengkes/govmomi/vim25/types"

// Reference is the interface that is implemented by all the managed objects
// defined in this package. It specifies that these managed objects have a
// function that returns the managed object reference to themselves.
type Reference interface {
	Reference() types.ManagedObjectReference
}
