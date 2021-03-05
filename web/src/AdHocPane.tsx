import {useTabNav} from "./TabNav"
import {LogUpdateAction, LogUpdateEvent, useLogStore} from "./LogStore"
import {ResourceName} from "./types"
import React, {Component, useEffect, useState} from "react"
import {Alert, combinedAlerts} from "./alerts"
import OverviewTabBar from "./OverviewTabBar"
import OverviewResourceBar from "./OverviewResourceBar"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import OverviewResourceDetails from "./OverviewResourceDetails"
import styled from "styled-components"
import {Color, Font, SizeUnit} from "./style-helpers"
import {ReactComponent} from "*.svg"
import SplitPane from "react-split-pane"

let PaneRoot = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background-color: ${Color.grayDark};
`

let Main = styled.div`
  display: flex;
  width: 100%;
  // In Safari, flex-basis "auto" squishes OverviewTabBar + OverviewResourceBar
  flex: 1 1 100%;
  overflow: hidden;
`

type AdHocPaneProps = {
  view: Proto.webviewView
}

export default function AdHocPane(props: AdHocPaneProps) {
  const logStore = useLogStore()
  let resources = props.view?.resources || []

  const [truncateCount, setTruncateCount] = useState<number>(0)

  // add a listener to rebuild alerts whenever a truncation event occurs
  // truncateCount is a dummy state variable to trigger a re-render to
  // simplify logic vs reconciliation between logStore + props
  useEffect(() => {
    const rebuildAlertsOnLogClear = (e: LogUpdateEvent) => {
      if (e.action === LogUpdateAction.truncate) {
        setTruncateCount(truncateCount + 1)
      }
    }

    logStore.addUpdateListener(rebuildAlertsOnLogClear)
    return () => logStore.removeUpdateListener(rebuildAlertsOnLogClear)
  }, [truncateCount])

  let alerts = resources.flatMap(r => combinedAlerts(r, logStore))

  // Hide the HTML element scrollbars, since this pane does all scrolling internally.
  // TODO(nick): Remove this when the old UI is deleted.
  useEffect(() => {
    document.documentElement.style.overflow = "hidden"
  })

  return (
    <PaneRoot>
      <OverviewResourceBar {...props} />
      <Main>
        <OverviewResourceSidebar {...props} name={"(all)"} />
        <SplitPane split="vertical" defaultSize="70%" style={{ position: 'static', overflow: 'visible scroll' }}>
          <OverviewResourceDetails resource={undefined} name={"(all)"} alerts={alerts} />
          <TiltfileEditor view={props.view}/>
        </SplitPane>
      </Main>
    </PaneRoot>
  )
}

function setTiltfileContent(content: string) {
  let url = `//${window.location.host}/api/write_tiltfile`

  fetch(url, {
    method: "post",
    body: content,
  })
    .then((res) => {
      res
        .json()
        .catch((err) => {
          console.error(err)
        })
    })
    .catch((err) => {
      console.error(err)
    })
}

type TiltfileEditorState = {
  bufferContent: string
  // this is a crappy solution, but probably good enough for a prototype.
  // basically: if someone edits the tiltfile outside of the webui, we want it to show up in the web ui.
  // however, if we update the tiltfile from the webui, and then the backend is just sending us that
  // same tiltfile again, then ignore the update - maybe we're already typing more!
  expectedServerContent: Set<string>
}

const EditorRoot = styled.div`
  height: 90%;
  background-color: ${Color.grayDark};
  display: flex;
  flex-flow: column;
  border-left: 5px solid ${Color.grayLightest};
`

const EditorHeader = styled.div`
  display: flex;
  justify-content: flex-end;
  background-color: ${Color.grayDarker};
  padding: ${SizeUnit(0.25)};
`

const SubmitButton = styled.button`
`

const TiltfileTextArea = styled.textarea`
  flex-grow: 1;
  width: 100%;
  background-color: transparent;
  color: ${Color.offWhite};
  padding: ${SizeUnit(0.5)};
  font-family: ${Font.monospace};
  border: none;
`

function TiltfileEditor(props: {view: Proto.webviewView}) {
  const viewContent = props.view.adhocTiltfileContents || ""
  const [state, setState] = useState<TiltfileEditorState>({bufferContent: viewContent, expectedServerContent: new Set(viewContent)})
  let activeContent = state.bufferContent
  if (!state.expectedServerContent.has(viewContent)) {
    state.expectedServerContent.add(viewContent)
    setState({bufferContent: viewContent, expectedServerContent: state.expectedServerContent})
    activeContent = viewContent
  }
  let setBufferContents = (content: string) => {
    // state.expectedServerContent.add(content)
    setState({bufferContent: content, expectedServerContent: state.expectedServerContent})
    // setTiltfileContent(content)
  }

  let updateServerContent = () => {
    state.expectedServerContent.add(activeContent)
    setState({bufferContent: activeContent, expectedServerContent: state.expectedServerContent})
    setTiltfileContent(activeContent)
  }

  return <EditorRoot>
    <EditorHeader>
      <SubmitButton onClick={e => { updateServerContent()}}>Submit</SubmitButton>
    </EditorHeader>
    <TiltfileTextArea onChange={e => { setBufferContents(e.target.value) }} value={activeContent} />
  </EditorRoot>
}