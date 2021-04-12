import React from "react"
import {useState} from "react"
import {mixinResetButtonStyle} from "./style-helpers"
import styled from "styled-components"
import Modal from "react-modal"

const RefreshButton = styled.button`
  ${mixinResetButtonStyle};
`

const DismissButton = styled.button`
  ${mixinResetButtonStyle};
`
const TiltVersionConsistencyPromptRoot = styled(Modal)`
`
export function TiltVersionConsistencyPrompt(props: {frontendVersion?: string, backendVersion?: string}) {
  const [dismissedVersion, setDismissedVersion] = useState<string | undefined>(undefined)

  const isOpen = !!props.frontendVersion && props.frontendVersion !== props.backendVersion && props.frontendVersion !== dismissedVersion

  const reload = () => window.location.reload()
  const dismiss = () => {setDismissedVersion(props.backendVersion)}

  return <Modal isOpen={isOpen}>
    Your browser was loaded from Tilt {props.frontendVersion},
    but your Tilt process is running Tilt {props.backendVersion}
    <RefreshButton onClick={reload}>Reload UI</RefreshButton>
    <DismissButton onClick={dismiss}>Dismiss</DismissButton>
  </Modal>
}