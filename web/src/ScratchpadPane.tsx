import {AllManifests, LogUpdateAction, LogUpdateEvent, ManifestSet, useLogStore} from "./LogStore"
import React, {useEffect, useState} from "react"
import {combinedAlerts} from "./alerts"
import OverviewResourceBar from "./OverviewResourceBar"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import styled from "styled-components"
import {Color, Font, FontSize, SizeUnit} from "./style-helpers"
import SplitPane from "react-split-pane"
import {useFilterSet} from "./logfilters"
import OverviewLogPane from "./OverviewLogPane"
import "./ScratchpadPane.scss"
import Editor, {Monaco, useMonaco} from "@monaco-editor/react"

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

type ScratchpadPaneProps = {
  view: Proto.webviewView
}

export default function ScratchpadPane(props: ScratchpadPaneProps) {
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
        .text()
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
  // this is a really bad solution, but probably good enough for a prototype.
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
const ScratchpadTitle = styled.div`
  font-size: ${FontSize.small};
  padding-right: ${SizeUnit(0.5)};
`
const SubmitButton = styled.button`
`

const TiltfileTextArea = styled(Editor)`

`

const SubmitShortcutIndicator = styled.div`
  color: ${Color.grayLightest};
  display: inline;
  font-size: ${FontSize.smallester};
`

// TODO(matt) figure out how to get types for editor or monaco
// @ts-ignore
function handleEditorDidMount(editor, monaco) {
  editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, () => {
    document.getElementById("tiltfile-submit-button")?.click()
  })
  editor.addAction({
    id: 'tilt-help',
    label: 'Help',
    keybindings: [ monaco.KeyCode.F2 ],
    precondition: null,
    keybindingContext: null,
    contextMenuGroupId: 'navigation',
    contextMenuOrder: 1.5,
    // @ts-ignore
    // TODO(matt) figure out what to do about unknown words
    run: (ed) => {
      const wordUnderCursor = ed.getModel().getWordAtPosition(ed.getPosition()).word
      const url = `https://docs.tilt.dev/api.html#api.${wordUnderCursor}`
      window.open(url, '_blank')
      return null
    }
  })
  // TODO(matt) fix or remove
  monaco.languages.registerCompletionItemProvider('python', {
    // @ts-ignore
    provideCompletionItems: function(model, position) {
      const word = model.getWordAtPosition(position)?.word
      console.log(`looking for hover for ${word}`)
      switch (word) {
        case 'local_resource':
          return {
            contents: [
              'local_resource(name, cmd, serve_cmd)'
            ]
          }
        default:
          return null
      }
    }
  })
}

function TiltfileEditor(props: {view: Proto.webviewView}) {
  const viewContent = props.view.scratchpadTiltfileContents || ""
  const [state, setState] = useState<TiltfileEditorState>({bufferContent: viewContent, expectedServerContent: new Set(viewContent)})
  let activeContent = state.bufferContent
  if (!state.expectedServerContent.has(viewContent)) {
    // the server had a version of the Tiltfile we haven't seen - assume it changed externally and throw out the buffer contents
    // at some point we'll want to prompt the user asking which one they want to keep
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
      <ScratchpadTitle>Tiltfile Scratchpad</ScratchpadTitle>
      <SubmitButton id="tiltfile-submit-button" onClick={e => { updateServerContent()}}>
        Update <SubmitShortcutIndicator>(⌘+↵)</SubmitShortcutIndicator>
      </SubmitButton>
    </EditorHeader>
    <TiltfileTextArea
      theme="vs-dark"
      defaultLanguage="python"
      onChange={(value, event) => { setBufferContents(value ?? "") }}
      value={activeContent}
      onMount={(editor, monaco) => handleEditorDidMount(editor, monaco)}
      /* https://microsoft.github.io/monaco-editor/api/interfaces/monaco.editor.istandaloneeditorconstructionoptions.html */
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