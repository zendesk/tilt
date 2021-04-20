import React, {Dispatch, SetStateAction, useContext} from 'react'

type ContextStateProps<T> = React.PropsWithChildren<{
  stateContext: React.Context<T>
  updateContext: React.Context<Dispatch<SetStateAction<T>>>
  value: T
  areEqual?: (a: T, b: T) => boolean
}>

export function NewStateContext<T>(val: T): [React.Context<T>, React.Context<Dispatch<SetStateAction<T>>>] {
  return [
    React.createContext<T>(val),
    React.createContext<Dispatch<SetStateAction<T>>>(() => {}),
  ]
}

function ContextStateProvider<T>(props: ContextStateProps<T>) {
  const [val, setVal] = React.useState(props.value)
  const wrappedSetVal: typeof setVal = React.useCallback((v) => {
    if (!props.areEqual) {
      return
    } else {
      // TODO(matt) - `instanceof Function` will blow up if T is a Function!
      const newVal = (v instanceof Function ? v(val) : val)
      if (props.areEqual(val, newVal)) {
        return
      }
      setVal(newVal)
    }
  }, [val, props.areEqual])

  return <props.stateContext.Provider value={val}>
    <props.updateContext.Provider value={wrappedSetVal}>
      {props.children}
    </props.updateContext.Provider>
  </props.stateContext.Provider>
}
