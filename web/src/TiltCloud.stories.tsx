import React from "react"
import { storiesOf } from "@storybook/react"
import TiltCloud from "./TiltCloud"
import {TiltCloudRole, TiltCloudTeamState} from "./types"

let schemeHost = "https://cloud.tilt.dev"

let userNoTeam = () => {
  return (
    <TiltCloud
    teamName={""}
    teamStatus={""}
    teamRole={""}
    teamID={""}
    userName={"laura"}
    schemeHost={schemeHost}
  />
  )
}

let specifiedAndUnknownTeamNoUser = () => {
  return (
    <TiltCloud
      teamName={"foobars"}
      teamStatus={TiltCloudTeamState.SpecifiedAndUnknown}
      teamRole={TiltCloudRole.Unknown}
      teamID={"1234567890"}
      userName={""}
      schemeHost={schemeHost}
    />
  )
}

let specifiedAndUnknownTeamUser = () => {
  return (
    <TiltCloud
      teamName={"foobars"}
      teamStatus={TiltCloudTeamState.SpecifiedAndUnknown}
      teamRole={TiltCloudRole.Unknown}
      teamID={"1234567890"}
      userName={"laura"}
      schemeHost={schemeHost}
    />
  )
}

let registeredTeamOwner = () => {
  return (
    <TiltCloud
      teamName={"foobars"}
      teamStatus={TiltCloudTeamState.SpecifiedAndRegistered}
      teamRole={TiltCloudRole.Owner}
      teamID={"1234567890"}
      userName={"laura"}
      schemeHost={schemeHost}
    />
  )
}

let registeredTeamUser = () => {
  return (
    <TiltCloud
      teamName={"foobars"}
      teamStatus={TiltCloudTeamState.SpecifiedAndRegistered}
      teamRole={TiltCloudRole.User}
      teamID={"1234567890"}
      userName={"freida"}
      schemeHost={schemeHost}
    />
  )
}

storiesOf("TiltCloud", module)
  .add("no team, signed in", userNoTeam)
  .add("unregistered team, not signed in", specifiedAndUnknownTeamNoUser)
  .add("unregistered team, signed in", specifiedAndUnknownTeamUser)
  .add("registered team, owner", registeredTeamOwner)
  .add("registered team, user", registeredTeamUser)
