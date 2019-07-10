import React, { PureComponent } from "react"
import { TiltDenStatus } from "./types"
import "./SailInfo.scss"

type SailProps = {
  sailEnabled: boolean
  sailUrl: string

  tiltDen: TiltDenStatus | null
}

class SailInfo extends PureComponent<SailProps> {
  static newSailRoom() {
    let url = `//${window.location.host}/api/sail`

    fetch(url, {
      method: "post",
      body: "",
    })
  }

  render() {
    if (this.props.sailEnabled) {
      if (this.props.sailUrl) {
        return (
          <span className="SailInfo">
            <a target="_blank" href={this.props.sailUrl}>
              Share this view!
            </a>
          </span>
        )
      }

      return (
        <span className="SailInfo">
          <button type="button" onClick={SailInfo.newSailRoom}>
            Share me!
          </button>
        </span>
      )
    }

    if (this.props.tiltDen) {
      return <span className="sail-url">Hi! {this.props.tiltDen.Msg} {this.props.tiltDen.Err}</span>
    }

    return <span className="sail-url">no tilt den</span>
  }
}

export default SailInfo
