export interface LibraryItem {
  id: number
  title: string
  content: string
  status: string
  priority: number
  created_at: string
  updated_at: string
  comments: LibraryComment[]
}

export interface LibraryComment {
  id: number
  item_id: number
  source: string
  message: string
  created_at: string
}
