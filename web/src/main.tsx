import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Penyedia } from './app/providers'
import { Rute } from './app/router'
import './styles/tokens.css'

const akar = document.getElementById('root')
if (!akar) throw new Error('Elemen #root tidak ditemukan')

createRoot(akar).render(
  <StrictMode>
    <Penyedia>
      <Rute />
    </Penyedia>
  </StrictMode>,
)
