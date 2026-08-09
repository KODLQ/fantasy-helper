export type DataStateStatus = 'loading' | 'ready' | 'empty' | 'error' | 'stale' | 'partial';

type DataStateProps = {
  status: Exclude<DataStateStatus, 'ready'>;
  message: string;
  retry?: () => void;
};

export function DataState({ status, message, retry }: DataStateProps) {
  const title = status === 'loading' ? 'Loading' : status === 'error' ? 'Something went wrong' : status === 'empty' ? 'Nothing to show' : status === 'stale' ? 'Showing stale data' : 'Showing partial data';
  return <div className={`data-state data-state-${status}`} role={status === 'error' ? 'alert' : undefined}>
    <strong>{title}</strong>
    <span>{message}</span>
    {retry && <button className="secondary-button" onClick={retry}>Try again</button>}
  </div>;
}
