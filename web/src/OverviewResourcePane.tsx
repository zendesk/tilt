import React from "react"
import styled from "styled-components"
import OverviewResourceBar from "./OverviewResourceBar"
import OverviewResourceDetails from "./OverviewResourceDetails"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import OverviewTabBar from "./OverviewTabBar"
import { Color } from "./style-helpers"
import { ResourceName } from "./types"

type OverviewResourcePaneProps = {
  name: string
  view: Proto.webviewView
}

let OverviewResourcePaneRoot = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background-color: ${Color.grayDark};
  max-height: 100%;
`

let Main = styled.div`
  display: flex;
  width: 100%;
  // In Safari, flex-basis "auto" squishes OverviewTabBar + OverviewResourceBar
  flex: 1 1 100%;
  overflow: hidden;
`

export default function OverviewResourcePane(props: OverviewResourcePaneProps) {
  let resources = props.view?.resources || []
  let name = props.name
  let r: Proto.webviewResource | undefined
  let all = name === "" || name === ResourceName.all
  if (!all) {
    r = resources.find((r) => r.name === name)
  }
  let selectedTab = ""
  if (all) {
    selectedTab = ResourceName.all
  } else if (r?.name) {
    selectedTab = r.name
  }

  return (
    <OverviewResourcePaneRoot>
      <OverviewTabBar selectedTab={selectedTab} />
      <OverviewResourceBar {...props} />
      <Main>
        <OverviewResourceSidebar {...props} />
        <OverviewResourceDetails resource={r} name={name} />
      </Main>
    </OverviewResourcePaneRoot>
  )
}