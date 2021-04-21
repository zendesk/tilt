import React, { Dispatch, SetStateAction, useContext } from "react"

// React.createContext requires a default value, but so does context.Provider.
// React.createContext's default value is only there in case you try to consume
// it outside of a provider. Some types don't have easy default values.
// This allows you to create a context w/ a default value of undefined and just
// have a runtime check that it's defined (via context.Provider)
export function useRequiredContext<T>(ctx: React.Context<T | undefined>): T {
  const ret = useContext(ctx)
  if (!ret) {
    throw new Error("")
  }
  return ret
}

type ContextStateProps<T> = React.PropsWithChildren<{
  stateContext: React.Context<T | undefined>
  setStateContext: React.Context<Dispatch<SetStateAction<T>>>
  value: T
  areEqual?: (a: T, b: T) => boolean
}>

export function NewStateContext<T>(): [
  React.Context<T | undefined>,
  React.Context<Dispatch<SetStateAction<T>>>
] {
  return [
    React.createContext<T | undefined>(undefined),
    React.createContext<Dispatch<SetStateAction<T>>>(() => {}),
  ]
}

// Does two things:
// 1. Combines useState and useContext into one API w/ two contexts
// 2. Skips setState if the new state is equal to the old state, to avoid rerenders
export function ContextStateProvider<T>(props: ContextStateProps<T>) {
  const [state, setState] = React.useState<T>(props.value)

  // wrap setVal with a guard to do nothing if there new value is equal to the old
  const wrappedSetVal: typeof setState = React.useCallback(
    (v) => {
      const areEqual = (a: T, b: T) =>
        props.areEqual ? props.areEqual(a, b) : a === b
      // TODO(matt) - `instanceof Function` is wrong if T is a Function!
      if (v instanceof Function) {
        setState((prevState) => {
          const newVal = v(prevState)
          if (!areEqual(newVal, state)) {
            return newVal
          } else {
            return prevState
          }
        })
      } else {
        if (!areEqual(v, state)) {
          setState(v)
        }
      }
    },
    [state, props.areEqual]
  )

  return (
    <props.stateContext.Provider value={state}>
      <props.setStateContext.Provider value={wrappedSetVal}>
        {props.children}
      </props.setStateContext.Provider>
    </props.stateContext.Provider>
  )
}
