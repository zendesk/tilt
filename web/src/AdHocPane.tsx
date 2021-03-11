import {AllManifests, LogUpdateAction, LogUpdateEvent, ManifestSet, useLogStore} from "./LogStore"
import React, {useEffect, useState} from "react"
import {combinedAlerts} from "./alerts"
import OverviewResourceBar from "./OverviewResourceBar"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import styled from "styled-components"
import {Color, Font, SizeUnit} from "./style-helpers"
import SplitPane from "react-split-pane"
import {useFilterSet} from "./logfilters"
import OverviewLogPane from "./OverviewLogPane"
import "./AdHocPane.scss"
import Editor from "@monaco-editor/react"

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
        <SplitPane split="vertical" defaultSize="70%" minSize="30%" style={{ position: 'static' }} pane1Style={{ display: 'flex', flexFlow: 'column', overflow: 'hidden' }} pane2Style={{ display: 'flex'}}>
          <CenterPane manifests={AllManifests} />
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
  //height: 90%;
  width: 100%;
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

const TiltfileTextArea = styled(Editor)`

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
    <TiltfileTextArea
      theme="vs-dark"
      defaultLanguage="python"
      onChange={(value, event) => { setBufferContents(value ?? "") }}
      defaultValue={activeContent}
      options={{
        lineNumbers: "off",
        folding: false,
        wordWrap: "on",
        minimap: {
          enabled: false,
        },
      }}
    />
  </EditorRoot>
}

let CenterPaneRoot = styled.div`
  display: flex;
  flex-grow: 1;
  flex-shrink: 1;
  flex-direction: column;
  overflow: hidden;
`

function CenterPane(
  props: { manifests: ManifestSet }
) {
  let filterSet = useFilterSet()

  return (
    <CenterPaneRoot>
      <OverviewLogPane manifests={props.manifests} filterSet={filterSet} />
    </CenterPaneRoot>
  )
}