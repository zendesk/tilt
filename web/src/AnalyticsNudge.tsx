import React, { Component } from "react"
import "./AnalyticsNudge.scss"

const nudgeTimeoutMs = 15000
const reqInProgMsg = "Okay, making it so..."
const successOptInElem = (): JSX.Element => {
    return (
        <p>
            Thanks for helping us improve Tilt for you and everyone! (You can change
            your mind by running <pre>tilt analytics opt out</pre> in your terminal.)
        </p>
    )
}
const successOptOutElem = (): JSX.Element => {
    return (
        <p>
            Okay, opting you out of telemetry. If you change your mind, you can run{" "}
            <pre>tilt analytics opt in</pre> in your terminal.
        </p>
    )
}
const errorElem = (respBody: string): JSX.Element => {
    return (
        <p>
            Oh no, something went wrong! Request failed with: <pre>{respBody}</pre> (
            <a
                href="https://tilt.dev/contact"
                target="_blank"
                rel="noopener noreferrer"
            >
                contact us
            </a>
            )
        </p>
    )
}

type AnalyticsNudgeProps = {
    guide?: Proto.webviewGuide
}

type AnalyticsNudgeState = {
    requestMade: boolean
    optIn: boolean
    responseCode: number
    responseBody: string
    dismissed: boolean
}

class AnalyticsNudge extends Component<
AnalyticsNudgeProps,
      AnalyticsNudgeState
> {
    constructor(props: AnalyticsNudgeProps) {
        super(props)

        this.state = {
            requestMade: false,
            optIn: false,
            responseCode: 0,
            responseBody: "",
            dismissed: false,
        }
    }

    guideChoice(choice: string) {
        let url = `//${window.location.host}/api/guide/choice`

        let payload = { choice: choice }
        fetch(url, {
            method: "post",
            body: JSON.stringify(payload),
        })
    }

    analyticsOpt(optIn: boolean) {
        let url = `//${window.location.host}/api/analytics_opt`

        let payload = { opt: optIn ? "opt-in" : "opt-out" }

        this.setState({ requestMade: true, optIn: optIn })

        fetch(url, {
            method: "post",
            body: JSON.stringify(payload),
        }).then((response: Response) => {
            response.text().then((body: string) => {
                this.setState({
                    responseCode: response.status,
                    responseBody: body,
                })
            })

            if (response.status === 200) {
                // if we successfully recorded the choice, dismiss the nudge after a few seconds
                setTimeout(() => {
                    this.setState({ dismissed: true })
                }, nudgeTimeoutMs)
            }
        })
    }

    messageElem(): JSX.Element {
        return (
            <>
                <p>
                    {this.props.guide?.message}
                </p>

                <span>
                    <button
                        className="AnalyticsNudge-button"
                        onClick={() => this.guideChoice("0")}
                    >
                        Next
                    </button>
                </span>
            </>
        )
    }
    render() {
        let classes = ["AnalyticsNudge"]
        if (this.props.guide?.message) {
            classes.push("is-visible")
        }

        return <section className={classes.join(" ")}>{this.messageElem()}</section>
    }
}

export default AnalyticsNudge
