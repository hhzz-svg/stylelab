import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import ToastHost from './components/ToastHost'
import ConfirmHost from './components/ConfirmDialog'
import './styles.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <ToastHost />
      <ConfirmHost />
      <App />
    </BrowserRouter>
  </React.StrictMode>,
)
