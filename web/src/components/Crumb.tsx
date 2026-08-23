import { Link } from 'react-router-dom'
import { projectName } from '../projectCache'

/** 面包屑：作品集 / {作品名} / 当前页。 */
export default function Crumb({ projectId, current }: { projectId?: string; current: string }) {
  return (
    <nav className="crumb" aria-label="路径">
      <Link to="/">作品集</Link>
      {projectId ? (
        <>
          <span className="crumb-sep">/</span>
          <Link to={`/p/${projectId}`}>{projectName(projectId)}</Link>
        </>
      ) : null}
      <span className="crumb-sep">/</span>
      <strong>{current}</strong>
    </nav>
  )
}
