import React from "react"
import styled from "styled-components"
import { incr } from "./analytics"
import LogStore, {ManifestSet, useLogStore} from "./LogStore"
import {
  AnimDuration,
  Color,
  FontSize,
  mixinResetButtonStyle,
} from "./style-helpers"
import { ResourceName } from "./types"

const ClearLogsButton = styled.button`
  ${mixinResetButtonStyle};
  margin-left: auto;
  font-size: ${FontSize.small};
  color: ${Color.white};
  transition: color ${AnimDuration.default} ease;

  &:hover {
    color: ${Color.blue};
  }
`

export interface ClearLogsProps {
  resources: ManifestSet
}

export const clearLogs = (
  logStore: LogStore,
  resources: ManifestSet,
  action: string
) => {
  let spans: { [key: string]: Proto.webviewLogSpan }
  spans = logStore.spansForManifests(resources)
  incr("ui.web.clearLogs", { action, all: (resources.type === 'all').toString() })
  logStore.removeSpans(Object.keys(spans))
}

const ClearLogs: React.FC<ClearLogsProps> = ({ resources }) => {
  const logStore = useLogStore()
  const label = resources.type === 'all' ? "Clear All Logs" : "Clear Logs"

  return (
    <ClearLogsButton onClick={() => clearLogs(logStore, resources, "click")}>
      {label}
    </ClearLogsButton>
  )
}

export default ClearLogs
