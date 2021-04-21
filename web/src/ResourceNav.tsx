import { History } from "history"
import React from "react"
import { matchPath, useHistory } from "react-router-dom"
import {
  ContextStateProvider,
  NewStateContext,
  useRequiredContext,
} from "./ContextState"
import PathBuilder, { usePathBuilder } from "./PathBuilder"
import { ResourceName } from "./types"

// Resource navigation semantics.
// 1. standardizes navigation
// 2. saves components from having to jump through hoops to get history + pathbuilder
export type ResourceNav = {
  validResources: string[]
  pb: PathBuilder
  history: History
}

function areResourceNavEqual(a: ResourceNav, b: ResourceNav): boolean {
  return (
    a.pb === b.pb &&
    a.history === b.history &&
    a.validResources.length === b.validResources.length &&
    a.validResources.every((e, i) => b.validResources[i] === e)
  )
}

const [
  resourceNavContext,
  setResourceNavContext,
] = NewStateContext<ResourceNav>()

export function useResourceNav(): ResourceNav {
  return useRequiredContext(resourceNavContext)
}

export const ResourceNavContextConsumer = resourceNavContext.Consumer
export const SetResourceNavContextConsumer = setResourceNavContext.Consumer

export function openResource(name: string, nav: ResourceNav) {
  name = name || ResourceName.all
  let url = nav.pb.encpath`/r/${name}/overview`

  nav.history.push(url)
}

export function selectedResource(
  nav: ResourceNav
): { selectedResource: string; invalidResource: string } {
  let selectedResource = ""
  let invalidResource = ""

  let matchResource = matchPath(nav.history.location.pathname, {
    path: nav.pb.path("/r/:name"),
  })
  let candidateResource = decodeURIComponent(
    (matchResource?.params as any)?.name || ""
  )

  if (candidateResource) {
    if (
      nav.validResources.includes(candidateResource) ||
      candidateResource == ResourceName.all
    ) {
      selectedResource = candidateResource
    }
  }
  if (!selectedResource) {
    invalidResource = candidateResource
  }

  return {
    selectedResource: selectedResource,
    invalidResource: invalidResource,
  }
}

export function ResourceNavProvider(props: React.PropsWithChildren<{validResources?: string[]}>) {
  const pb = usePathBuilder()
  const history = useHistory()
  return (
    <ContextStateProvider<ResourceNav>
      stateContext={resourceNavContext}
      setStateContext={setResourceNavContext}
      value={{ pb: pb, history: history, validResources: props.validResources ?? [] }}
      areEqual={areResourceNavEqual}
    >
      {props.children}
    </ContextStateProvider>
  )
}
