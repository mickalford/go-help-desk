import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { listClients, createClient, updateClient, deleteClient } from '@/api/admin'
import { extractError } from '@/api/client'
import { Layout } from '@/components/Layout'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { PlusIcon, PencilIcon, CheckIcon, XIcon, TrashIcon } from 'lucide-react'
import type { Client } from '@/api/types'

interface EditState {
  name: string
  domain: string
  notes: string
}

function EditRow({
  client,
  onSave,
  onCancel,
}: {
  client: Client
  onSave: (patch: EditState) => Promise<void>
  onCancel: () => void
}) {
  const [draft, setDraft] = useState<EditState>({
    name: client.name,
    domain: client.domain,
    notes: client.notes ?? '',
  })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function handleSave() {
    if (!draft.name.trim() || !draft.domain.trim()) return
    setSaving(true)
    try {
      await onSave(draft)
    } catch (err) {
      setError(extractError(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <tr className="bg-blue-50">
        <td className="px-4 py-2">
          <Input
            value={draft.name}
            onChange={(e) => setDraft(d => ({ ...d, name: e.target.value }))}
            className="h-7 text-sm"
          />
        </td>
        <td className="px-4 py-2">
          <Input
            value={draft.domain}
            onChange={(e) => setDraft(d => ({ ...d, domain: e.target.value }))}
            className="h-7 text-sm font-mono"
            placeholder="acme.com"
          />
        </td>
        <td className="px-4 py-2">
          <Input
            value={draft.notes}
            onChange={(e) => setDraft(d => ({ ...d, notes: e.target.value }))}
            className="h-7 text-sm"
            placeholder="Optional notes"
          />
        </td>
        <td className="px-4 py-2 text-gray-400">—</td>
        <td className="px-4 py-2 text-right">
          <div className="flex items-center justify-end gap-2">
            <Button size="sm" onClick={handleSave} disabled={saving || !draft.name.trim() || !draft.domain.trim()}>
              <CheckIcon className="h-3.5 w-3.5" />
            </Button>
            <Button size="sm" variant="ghost" onClick={onCancel} disabled={saving}>
              <XIcon className="h-3.5 w-3.5" />
            </Button>
          </div>
          {error && <p className="mt-1 text-xs text-red-600 text-right">{error}</p>}
        </td>
      </tr>
    </>
  )
}

export function ClientsPage() {
  const qc = useQueryClient()
  const [newName, setNewName] = useState('')
  const [newDomain, setNewDomain] = useState('')
  const [newNotes, setNewNotes] = useState('')
  const [createError, setCreateError] = useState('')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [pendingDelete, setPendingDelete] = useState<Client | null>(null)

  const { data: clients = [], isLoading } = useQuery({
    queryKey: ['admin', 'clients'],
    queryFn: listClients,
  })

  const createMutation = useMutation({
    mutationFn: () =>
      createClient({ name: newName.trim(), domain: newDomain.trim().toLowerCase(), notes: newNotes.trim() || undefined }),
    onSuccess: () => {
      setNewName('')
      setNewDomain('')
      setNewNotes('')
      setCreateError('')
      qc.invalidateQueries({ queryKey: ['admin', 'clients'] })
    },
    onError: (err) => setCreateError(extractError(err)),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: EditState }) =>
      updateClient(id, { name: patch.name.trim(), domain: patch.domain.trim().toLowerCase(), notes: patch.notes.trim() || undefined }),
    onSuccess: () => {
      setEditingId(null)
      qc.invalidateQueries({ queryKey: ['admin', 'clients'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteClient(id),
    onSuccess: () => {
      setPendingDelete(null)
      qc.invalidateQueries({ queryKey: ['admin', 'clients'] })
    },
  })

  const canCreate = newName.trim() !== '' && newDomain.trim() !== ''

  return (
    <Layout>
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Clients</h1>
          <p className="mt-1 text-sm text-gray-500">
            Manage client organisations. The email domain is used by the email bridge to automatically
            associate inbound tickets with the correct client.
          </p>
        </div>

        {/* Create form */}
        <div className="rounded-lg border bg-white p-4">
          <p className="mb-3 text-sm font-medium text-gray-700">Add client</p>
          <div className="flex items-end gap-3 flex-wrap">
            <div className="flex flex-col gap-1">
              <label className="text-xs text-gray-500">Company name</label>
              <Input
                placeholder="Acme Corp"
                value={newName}
                onChange={(e) => { setNewName(e.target.value); setCreateError('') }}
                onKeyDown={(e) => { if (e.key === 'Enter' && canCreate) createMutation.mutate() }}
                className="w-48"
              />
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-xs text-gray-500">Email domain</label>
              <Input
                placeholder="acme.com"
                value={newDomain}
                onChange={(e) => { setNewDomain(e.target.value); setCreateError('') }}
                onKeyDown={(e) => { if (e.key === 'Enter' && canCreate) createMutation.mutate() }}
                className="w-44 font-mono text-sm"
              />
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-xs text-gray-500">Notes (optional)</label>
              <Input
                placeholder="e.g. Primary contact is Jane"
                value={newNotes}
                onChange={(e) => setNewNotes(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter' && canCreate) createMutation.mutate() }}
                className="w-64"
              />
            </div>
            <Button
              onClick={() => createMutation.mutate()}
              disabled={!canCreate || createMutation.isPending}
            >
              <PlusIcon className="mr-2 h-4 w-4" />
              {createMutation.isPending ? 'Adding…' : 'Add'}
            </Button>
          </div>
          {createError && <p className="mt-2 text-sm text-red-600">{createError}</p>}
        </div>

        {isLoading ? (
          <div className="flex justify-center py-12"><Spinner /></div>
        ) : (
          <div className="rounded-lg border bg-white overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 text-xs text-gray-500 uppercase">
                <tr>
                  <th className="px-4 py-3 text-left">Name</th>
                  <th className="px-4 py-3 text-left">Domain</th>
                  <th className="px-4 py-3 text-left">Notes</th>
                  <th className="px-4 py-3 text-left">Added</th>
                  <th className="px-4 py-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {clients.map((c) =>
                  editingId === c.id ? (
                    <EditRow
                      key={c.id}
                      client={c}
                      onSave={(patch) => updateMutation.mutateAsync({ id: c.id, patch })}
                      onCancel={() => setEditingId(null)}
                    />
                  ) : (
                    <tr key={c.id} className="hover:bg-gray-50">
                      <td className="px-4 py-3 font-medium text-gray-900">{c.name}</td>
                      <td className="px-4 py-3 font-mono text-gray-600">{c.domain}</td>
                      <td className="px-4 py-3 text-gray-500 max-w-xs truncate">{c.notes ?? '—'}</td>
                      <td className="px-4 py-3 text-gray-400 whitespace-nowrap">
                        {new Date(c.created_at).toLocaleDateString()}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <div className="flex items-center justify-end gap-2">
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => setEditingId(c.id)}
                          >
                            <PencilIcon className="h-3.5 w-3.5" />
                          </Button>
                          <Button
                            size="sm"
                            variant="outline"
                            className="text-red-600 border-red-200 hover:bg-red-50"
                            onClick={() => setPendingDelete(c)}
                            disabled={deleteMutation.isPending}
                          >
                            <TrashIcon className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  )
                )}
                {clients.length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-4 py-8 text-center text-gray-400">
                      No clients yet. Add one above.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <ConfirmDialog
        open={pendingDelete !== null}
        onOpenChange={(open) => { if (!open) setPendingDelete(null) }}
        title={`Delete client "${pendingDelete?.name ?? ''}"?`}
        description="This removes the client record. Existing tickets that reference this client will retain the ID but the name will no longer resolve."
        confirmLabel="Delete"
        isPending={deleteMutation.isPending}
        onConfirm={() => { if (pendingDelete) deleteMutation.mutate(pendingDelete.id) }}
      />
    </Layout>
  )
}
