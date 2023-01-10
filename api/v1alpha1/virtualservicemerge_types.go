/*
 * Copyright 2021 - now, the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *       https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	"fmt"

	"github.com/monimesl/operator-helper/reconciler"
	"istio.io/api/networking/v1alpha3"
	alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

// VirtualServiceMergeList contains a list of VirtualServiceMerge
type VirtualServiceMergeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualServiceMerge `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualServiceMerge{}, &VirtualServiceMergeList{})
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type VirtualServiceMerge struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualServiceMergeSpec   `json:"spec,omitempty"`
	Status VirtualServicePatchStatus `json:"status,omitempty"`
}

func (in *VirtualServiceMerge) AddTcpRoutes(target *alpha3.VirtualService) {
	targetRoutes := target.Spec.Tcp
outer:
	for _, pRoute := range in.Spec.Patch.Tcp {
		for i, tRoute := range targetRoutes {
			if tcpMatchesEqual(tRoute.Match, pRoute.Match) {
				targetRoutes[i] = pRoute // replace
				continue outer
			}
		}
		// add
		targetRoutes = append(targetRoutes, pRoute)
	}
	target.Spec.Tcp = targetRoutes
}

func (in *VirtualServiceMerge) RemoveTcpRoutes(target *alpha3.VirtualService) {
	targetRoutes := target.Spec.Tcp
outer:
	for _, pRoute := range in.Spec.Patch.Tcp {
		for i, tRoute := range targetRoutes {
			if tcpMatchesEqual(tRoute.Match, pRoute.Match) {
				// remove the route
				targetRoutes = append(targetRoutes[0:i], targetRoutes[i+1:]...)
				continue outer
			}
		}
	}
	target.Spec.Tcp = targetRoutes
}

func tcpMatchesEqual(sourceMatches []*v1alpha3.L4MatchAttributes, match2 []*v1alpha3.L4MatchAttributes) bool {
	for _, sM := range sourceMatches {
		for _, cM := range match2 {
			if sM.Port == cM.Port {
				// we treat port equality as equal
				return true
			}
		}
	}
	return false
}

func (in *VirtualServiceMerge) AddTlsRoutes(target *alpha3.VirtualService) {
	targetRoutes := target.Spec.Tls
outer:
	for _, pRoute := range in.Spec.Patch.Tls {
		for i, tRoute := range targetRoutes {
			if tlsMatchesEqual(tRoute.Match, pRoute.Match) {
				targetRoutes[i] = pRoute // replace
				continue outer
			}
		}
		// add
		targetRoutes = append(targetRoutes, pRoute)
	}
	target.Spec.Tls = targetRoutes
}

func (in *VirtualServiceMerge) RemoveTlsRoutes(target *alpha3.VirtualService) {
	targetRoutes := target.Spec.Tls
outer:
	for _, pRoute := range in.Spec.Patch.Tls {
		for i, tRoute := range targetRoutes {
			if tlsMatchesEqual(tRoute.Match, pRoute.Match) {
				// remove the route
				targetRoutes = append(targetRoutes[0:i], targetRoutes[i+1:]...)
				continue outer
			}
		}
	}
	target.Spec.Tls = targetRoutes
}

func tlsMatchesEqual(sourceMatches []*v1alpha3.TLSMatchAttributes, match2 []*v1alpha3.TLSMatchAttributes) bool {
	for _, sM := range sourceMatches {
		for _, cM := range match2 {
			if sM.Port == cM.Port {
				// we treat port equality as equal
				return true
			}
		}
	}
	return false
}

func (in *VirtualServiceMerge) AddHttpRoutes(ctx reconciler.Context, target *alpha3.VirtualService) {
	targetRoutes := target.Spec.Http
	patchRoutes := in.generateHttpRoutes()
outer:
	for _, pRoute := range patchRoutes {
		for i, tRoute := range targetRoutes {
			if tRoute.Name == pRoute.Name {
				targetRoutes[i] = pRoute // replace
				continue outer
			}
		}
		// prepend
		targetRoutes = append([]*v1alpha3.HTTPRoute{pRoute}, targetRoutes...)
	}
	target.Spec.Http = sanitizeRoutes(ctx, targetRoutes)
}

func (in *VirtualServiceMerge) RemoveHttpRoutes(ctx reconciler.Context, target *alpha3.VirtualService) {
	targetRoutes := target.Spec.Http
	patchRoutes := in.generateHttpRoutes()
outer:
	for _, pRoute := range patchRoutes {
		for i, tRoute := range targetRoutes {
			if tRoute.Name == pRoute.Name {
				// remove the route
				targetRoutes = append(targetRoutes[0:i], targetRoutes[i+1:]...)
				continue outer
			}
		}
	}
	target.Spec.Http = sanitizeRoutes(ctx, targetRoutes)
}

func sanitizeRoutes(ctx reconciler.Context, routes []*v1alpha3.HTTPRoute) []*v1alpha3.HTTPRoute {
	routeByNamePart := map[string]bool{}
	for i, route := range routes {
		name := route.Name
		if !routeByNamePart[name] {
			routeByNamePart[name] = true
			continue
		}
		ctx.Logger().Info("Dropping the duplicate route name prefix",
			"prefix", name, "route", route.Name)
		if (i + 1) < len(routes) { // the ith position is not the last
			routes = append(routes[0:i], routes[i+1:]...)
		} else {
			routes = routes[0:i]
		}
	}
	return routes
}

func parsePrecedence(ctx reconciler.Context, name string) (string, int64) {
	return "", 0
}

func (in *VirtualServiceMerge) generateHttpRoutes() []*v1alpha3.HTTPRoute {
	routes := make([]*v1alpha3.HTTPRoute, len(in.Spec.Patch.Http))
	for i, r := range in.Spec.Patch.Http {
		if r.Name == "" {
			r.Name = fmt.Sprintf("%s-%d", in.Name, i)
		}
		routes[i] = r
	}
	return routes
}
