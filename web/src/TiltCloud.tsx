import React, {PureComponent} from "react"
import styled from "styled-components"
import {Color, SizeUnit, Width} from "./style-helpers"
import {TiltCloudTeamState} from "./types"

export type TiltCloudProps = {
  teamID: string | undefined
  userName: string | undefined
  teamName: string | undefined
  teamStatus: string | undefined
  teamRole: string | undefined
  schemeHost: string | undefined
}

let TiltCloudAccount = styled.button`
  background-color: transparent;
  border: 0 none;
  display: block;
  align-items: center;
`

const loggedInClassName = "loggedIn"
let TiltCloudAccountIcon = styled.div`
  background-color: ${Color.grayLight};
  width: ${Width.badge}px;
  height: ${Width.badge}px;
  border-radius: ${Width.badge}px;
  float: left;
  
  &.${loggedInClassName} {
    background-color: ${Color.green};
  }
`
let TiltCloudAccountText = styled.span`
  color: ${Color.white};
  margin-right: ${SizeUnit(0.25)};
`
let TiltCloudTeamButton = styled.button`
  background-color: transparent;
  border: 0 none;
  display: block;
  align-items: center;
  color: ${Color.white};
`

type TiltCloudTeamProps = {
  teamName: string | undefined
  teamID: string | undefined
  teamStatus: string | undefined
  teamRole: string | undefined
  tiltCloudSchemeHost: string | undefined
}

class TiltCloudTeam extends PureComponent<TiltCloudTeamProps> {
  teamLink() {
    // TODO build a team page at /team/$teamid
    let url = this.props.tiltCloudSchemeHost + "/team/" + this.props.teamID + "/snapshots"
    return <a href={url} target="_blank"><TiltCloudTeamButton>{this.props.teamName}</TiltCloudTeamButton></a>
  }

  render() {
    switch (this.props.teamStatus) {
      case TiltCloudTeamState.Unknown:
        return null
      case TiltCloudTeamState.Unspecified:
        return null
      case TiltCloudTeamState.SpecifiedAndUnknown:
        return this.teamLink()
      case TiltCloudTeamState.SpecifiedAndRegistered:
        return this.teamLink()
      case TiltCloudTeamState.SpecifiedAndUnregistered:
        // TODO(matt) add registration nudge
        return this.teamLink()
      default:
        return null
    }
  }
}


class TiltCloud extends PureComponent<TiltCloudProps> {
  render() {
    return (
      <div>
        <TiltCloudAccountIcon className={this.loggedInStatus()}>&nbsp;</TiltCloudAccountIcon>
        <p>
          <TiltCloudAccountText>{this.props.userName}</TiltCloudAccountText>
          <TiltCloudTeam teamID={this.props.teamID} teamName={this.props.teamName} teamRole={this.props.teamRole} teamStatus={this.props.teamStatus} tiltCloudSchemeHost={this.props.schemeHost}/>
        </p>
      </div>
    )
  }

  loggedInStatus() {
    if (this.props.userName) {
      return loggedInClassName
    } else {
      return ""
    }
  }
}

export default TiltCloud
