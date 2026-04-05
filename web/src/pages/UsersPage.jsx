import React, { useEffect, useState } from 'react'
import { Users, Plus, Trash2, Shield, User, Edit2, Key, Save } from 'lucide-react'
import { useUserStore, useAuthStore, useUIStore } from '../store'
import Modal from '../components/ui/Modal'
import clsx from 'clsx'

export default function UsersPage() {
  const { users, fetchUsers, createUser, deleteUser } = useUserStore()
  const { user: currentUser, logout } = useAuthStore()
  const { showToast } = useUIStore()
  const [showNew, setShowNew] = useState(false)
  const [editUser, setEditUser] = useState(null) // 编辑中的用户
  const [showEditSelf, setShowEditSelf] = useState(false) // 编辑自身
  const [form, setForm] = useState({ username: '', password: '', role: 'user' })
  const [selfForm, setSelfForm] = useState({ username: '', old_password: '', new_password: '' })

  useEffect(() => { fetchUsers() }, [])

  // 创建用户
  const handleCreate = async () => {
    if (!form.username.trim() || !form.password) {
      showToast('请填写用户名和密码', 'error'); return
    }
    try {
      await createUser(form.username.trim(), form.password, form.role)
      showToast('用户创建成功', 'success')
      setShowNew(false)
      setForm({ username: '', password: '', role: 'user' })
    } catch (err) { showToast(err.message, 'error') }
  }

  // 删除用户
  const handleDelete = async (id) => {
    if (!confirm('确定要删除该用户吗？其所有数据将一并删除。')) return
    try {
      await deleteUser(id)
      showToast('用户已删除', 'success')
    } catch (err) { showToast(err.message, 'error') }
  }

  // 打开编辑弹窗
  const openEdit = (u) => {
    setEditUser(u)
    setForm({ username: u.username, password: '', role: u.role })
  }

  // 保存编辑
  const handleSaveEdit = async () => {
    if (!form.username.trim()) { showToast('用户名不能为空', 'error'); return }
    try {
      const body = { username: form.username.trim(), role: form.role }
      if (form.password) body.password = form.password
      const res = await fetch(`/api/auth/users/${editUser.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${useAuthStore.getState().token}`
        },
        body: JSON.stringify(body),
      })
      if (!res.ok) {
        const err = await res.json()
        throw new Error(err.error || '更新失败')
      }
      showToast('用户更新成功', 'success')
      setEditUser(null)
      fetchUsers()
    } catch (err) { showToast(err.message, 'error') }
  }

  // admin 修改自身
  const openEditSelf = () => {
    setSelfForm({ username: currentUser.username, old_password: '', new_password: '' })
    setShowEditSelf(true)
  }

  const handleSaveSelf = async () => {
    try {
      const token = useAuthStore.getState().token
      const headers = { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` }

      // 修改用户名（通过 admin 接口）
      if (selfForm.username !== currentUser.username) {
        const res = await fetch(`/api/auth/users/${currentUser.id}`, {
          method: 'PUT', headers,
          body: JSON.stringify({ username: selfForm.username, role: 'admin' }),
        })
        if (!res.ok) { const e = await res.json(); throw new Error(e.error) }
      }

      // 修改密码
      if (selfForm.old_password && selfForm.new_password) {
        const res = await fetch('/api/auth/password', {
          method: 'PUT', headers,
          body: JSON.stringify({ old_password: selfForm.old_password, new_password: selfForm.new_password }),
        })
        if (!res.ok) { const e = await res.json(); throw new Error(e.error) }
      }

      showToast('修改成功，请重新登录', 'success')
      setShowEditSelf(false)

      // 如果用户名或密码改了，需要重新登录
      if (selfForm.username !== currentUser.username || selfForm.new_password) {
        setTimeout(() => logout(), 1500)
      } else {
        fetchUsers()
      }
    } catch (err) { showToast(err.message, 'error') }
  }

  return (
    <div className="flex flex-col h-full">
      <div className="px-4 pt-6 pb-4 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">用户管理</h1>
          <p className="text-xs text-gray-500 mt-1">{users.length} 个用户</p>
        </div>
        <div className="flex gap-2">
          <button onClick={openEditSelf}
            className="flex items-center gap-1.5 py-2 px-3 text-sm rounded-xl
                       border border-surface-border text-gray-300 hover:bg-surface-hover transition-colors">
            <Key size={14} /> 修改账户
          </button>
          <button onClick={() => { setForm({ username: '', password: '', role: 'user' }); setShowNew(true) }}
            className="btn-primary flex items-center gap-2 py-2 px-4 text-sm">
            <Plus size={16} /> 新建
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-4 space-y-2">
        {users.map(u => (
          <div key={u.id} className="card p-4 flex items-center gap-3">
            <div className={clsx(
              'w-10 h-10 rounded-2xl flex items-center justify-center flex-shrink-0',
              u.role === 'admin'
                ? 'bg-gradient-to-br from-amber-500/20 to-orange-500/20 border border-amber-500/20'
                : 'bg-surface border border-surface-border'
            )}>
              {u.role === 'admin'
                ? <Shield size={18} className="text-amber-400" />
                : <User size={18} className="text-gray-400" />
              }
            </div>

            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="font-medium text-sm">{u.username}</span>
                <span className={clsx('text-[10px] px-1.5 py-0.5 rounded-full border',
                  u.role === 'admin'
                    ? 'bg-amber-500/15 text-amber-300 border-amber-500/30'
                    : 'bg-surface text-gray-400 border-surface-border'
                )}>
                  {u.role === 'admin' ? '管理员' : '用户'}
                </span>
              </div>
              <p className="text-xs text-gray-500">
                创建于 {new Date(u.created_at).toLocaleDateString('zh-CN')}
              </p>
            </div>

            <div className="flex gap-1">
              {/* 编辑按钮（自身通过"修改账户"按钮编辑） */}
              {u.id !== currentUser?.id && (
                <>
                  <button onClick={() => openEdit(u)}
                    className="p-2 text-gray-500 hover:text-white transition-colors rounded-lg">
                    <Edit2 size={15} />
                  </button>
                  <button onClick={() => handleDelete(u.id)}
                    className="p-2 text-gray-500 hover:text-red-400 transition-colors rounded-lg">
                    <Trash2 size={15} />
                  </button>
                </>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* 新建用户弹窗 */}
      <Modal open={showNew} onClose={() => setShowNew(false)} title="创建用户">
        <div className="space-y-4">
          <div>
            <label className="block text-xs text-gray-400 mb-1.5">用户名 *</label>
            <input className="w-full input-base" value={form.username}
              onChange={e => setForm(f => ({ ...f, username: e.target.value }))}
              placeholder="输入用户名" />
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1.5">密码 *</label>
            <input type="password" className="w-full input-base" value={form.password}
              onChange={e => setForm(f => ({ ...f, password: e.target.value }))}
              placeholder="输入密码" />
          </div>
          <div className="flex gap-3 pt-2">
            <button onClick={() => setShowNew(false)}
              className="flex-1 py-3 rounded-xl border border-surface-border text-gray-300
                         hover:bg-surface-hover transition-colors">取消</button>
            <button onClick={handleCreate} className="flex-1 btn-primary py-3">创建</button>
          </div>
        </div>
      </Modal>

      {/* 编辑用户弹窗 */}
      <Modal open={!!editUser} onClose={() => setEditUser(null)}
        title={`编辑用户: ${editUser?.username}`}>
        <div className="space-y-4">
          <div>
            <label className="block text-xs text-gray-400 mb-1.5">用户名</label>
            <input className="w-full input-base" value={form.username}
              onChange={e => setForm(f => ({ ...f, username: e.target.value }))} />
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1.5">新密码（留空不修改）</label>
            <input type="password" className="w-full input-base" value={form.password}
              onChange={e => setForm(f => ({ ...f, password: e.target.value }))}
              placeholder="留空则不修改密码" />
          </div>
          <div className="flex gap-3 pt-2">
            <button onClick={() => setEditUser(null)}
              className="flex-1 py-3 rounded-xl border border-surface-border text-gray-300
                         hover:bg-surface-hover transition-colors">取消</button>
            <button onClick={handleSaveEdit} className="flex-1 btn-primary py-3">保存</button>
          </div>
        </div>
      </Modal>

      {/* 修改自身账户弹窗 */}
      <Modal open={showEditSelf} onClose={() => setShowEditSelf(false)} title="修改账户信息">
        <div className="space-y-4">
          <div>
            <label className="block text-xs text-gray-400 mb-1.5">用户名</label>
            <input className="w-full input-base" value={selfForm.username}
              onChange={e => setSelfForm(f => ({ ...f, username: e.target.value }))} />
          </div>
          <div className="border-t border-surface-border pt-4">
            <p className="text-xs text-gray-500 mb-3">修改密码（不需要修改则留空）</p>
            <div className="space-y-3">
              <div>
                <label className="block text-xs text-gray-400 mb-1.5">当前密码</label>
                <input type="password" className="w-full input-base" value={selfForm.old_password}
                  onChange={e => setSelfForm(f => ({ ...f, old_password: e.target.value }))}
                  placeholder="输入当前密码" />
              </div>
              <div>
                <label className="block text-xs text-gray-400 mb-1.5">新密码</label>
                <input type="password" className="w-full input-base" value={selfForm.new_password}
                  onChange={e => setSelfForm(f => ({ ...f, new_password: e.target.value }))}
                  placeholder="输入新密码" />
              </div>
            </div>
          </div>
          <div className="flex gap-3 pt-2">
            <button onClick={() => setShowEditSelf(false)}
              className="flex-1 py-3 rounded-xl border border-surface-border text-gray-300
                         hover:bg-surface-hover transition-colors">取消</button>
            <button onClick={handleSaveSelf} className="flex-1 btn-primary py-3">保存</button>
          </div>
        </div>
      </Modal>
    </div>
  )
}
