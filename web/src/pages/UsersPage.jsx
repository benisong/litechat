import React, { useEffect, useState } from 'react'
import { Users, Plus, Trash2, Shield, User, Edit2, Key, Coins } from 'lucide-react'
import { useUserStore, useAuthStore, useUIStore, useSettingsStore } from '../store'
import Modal from '../components/ui/Modal'
import clsx from 'clsx'

export default function UsersPage() {
  const { users, fetchUsers, createUser, deleteUser, updateBalance } = useUserStore()
  const { user: currentUser, logout } = useAuthStore()
  const { settings } = useSettingsStore()
  const { showToast } = useUIStore()
  const [showNew, setShowNew] = useState(false)
  const [editUser, setEditUser] = useState(null)
  const [showEditSelf, setShowEditSelf] = useState(false)
  const [form, setForm] = useState({ username: '', password: '', role: 'user' })
  const [selfForm, setSelfForm] = useState({ username: '', old_password: '', new_password: '' })

  // 充值弹窗
  const [balanceUser, setBalanceUser] = useState(null)
  const [balanceDelta, setBalanceDelta] = useState('')

  const isServiceMode = settings.service_mode === 'service'

  useEffect(() => { fetchUsers() }, [])

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

  const handleDelete = async (id) => {
    if (!confirm('确定要删除该用户吗？其所有数据将一并删除。')) return
    try {
      await deleteUser(id)
      showToast('用户已删除', 'success')
    } catch (err) { showToast(err.message, 'error') }
  }

  const openEdit = (u) => {
    setEditUser(u)
    setForm({ username: u.username, password: '', role: u.role })
  }

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

  const openEditSelf = () => {
    setSelfForm({ username: currentUser.username, old_password: '', new_password: '' })
    setShowEditSelf(true)
  }

  const handleSaveSelf = async () => {
    try {
      const token = useAuthStore.getState().token
      const headers = { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` }

      if (selfForm.username !== currentUser.username) {
        const res = await fetch(`/api/auth/users/${currentUser.id}`, {
          method: 'PUT', headers,
          body: JSON.stringify({ username: selfForm.username, role: 'admin' }),
        })
        if (!res.ok) { const e = await res.json(); throw new Error(e.error) }
      }

      if (selfForm.old_password && selfForm.new_password) {
        const res = await fetch('/api/auth/password', {
          method: 'PUT', headers,
          body: JSON.stringify({ old_password: selfForm.old_password, new_password: selfForm.new_password }),
        })
        if (!res.ok) { const e = await res.json(); throw new Error(e.error) }
      }

      showToast('修改成功，请重新登录', 'success')
      setShowEditSelf(false)

      if (selfForm.username !== currentUser.username || selfForm.new_password) {
        setTimeout(() => logout(), 1500)
      } else {
        fetchUsers()
      }
    } catch (err) { showToast(err.message, 'error') }
  }

  // 充值/扣费
  const handleBalance = async () => {
    const delta = parseInt(balanceDelta)
    if (isNaN(delta) || delta === 0) {
      showToast('请输入有效的积分数', 'error'); return
    }
    try {
      await updateBalance(balanceUser.id, delta)
      showToast(`${delta > 0 ? '充值' : '扣除'} ${Math.abs(delta)} 积分成功`, 'success')
      setBalanceUser(null)
      setBalanceDelta('')
      fetchUsers()
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
              {/* 用量统计（服务模式下显示） */}
              {isServiceMode && u.role !== 'admin' ? (
                <div className="flex items-center gap-3 text-xs text-gray-500 mt-0.5">
                  <span>余额: <span className={clsx('font-medium', u.balance > 0 ? 'text-green-400' : 'text-red-400')}>{u.balance}</span></span>
                  <span>消息: {u.total_messages || 0}</span>
                  <span>Token: {(u.total_tokens || 0).toLocaleString()}</span>
                </div>
              ) : (
                <p className="text-xs text-gray-500">
                  创建于 {new Date(u.created_at).toLocaleDateString('zh-CN')}
                </p>
              )}
            </div>

            <div className="flex gap-1">
              {u.id !== currentUser?.id && (
                <>
                  {/* 充值按钮（服务模式 + 非admin） */}
                  {isServiceMode && u.role !== 'admin' && (
                    <button onClick={() => { setBalanceUser(u); setBalanceDelta('') }}
                      className="p-2 text-gray-500 hover:text-amber-400 transition-colors rounded-lg">
                      <Coins size={15} />
                    </button>
                  )}
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

      {/* 充值/扣费弹窗 */}
      <Modal open={!!balanceUser} onClose={() => setBalanceUser(null)}
        title={`积分管理: ${balanceUser?.username}`}>
        {balanceUser && (
          <div className="space-y-4">
            <div className="flex items-center justify-between p-3 rounded-xl bg-surface border border-surface-border">
              <span className="text-sm text-gray-400">当前余额</span>
              <span className={clsx('text-lg font-bold', balanceUser.balance > 0 ? 'text-green-400' : 'text-red-400')}>
                {balanceUser.balance} 积分
              </span>
            </div>

            <div className="flex items-center justify-between p-3 rounded-xl bg-surface/50 border border-surface-border text-xs text-gray-500">
              <span>累计消息: {balanceUser.total_messages || 0} 条</span>
              <span>累计Token: {(balanceUser.total_tokens || 0).toLocaleString()}</span>
            </div>

            <div>
              <label className="block text-xs text-gray-400 mb-1.5">充值/扣除积分</label>
              <input type="number" className="w-full input-base" value={balanceDelta}
                onChange={e => setBalanceDelta(e.target.value)}
                placeholder="正数充值，负数扣除" />
              {balanceDelta && !isNaN(parseInt(balanceDelta)) && (
                <p className="text-xs text-gray-500 mt-1">
                  操作后余额: <span className="font-medium text-gray-300">
                    {balanceUser.balance + parseInt(balanceDelta)} 积分
                  </span>
                </p>
              )}
            </div>

            <div className="flex gap-3 pt-2">
              <button onClick={() => setBalanceUser(null)}
                className="flex-1 py-3 rounded-xl border border-surface-border text-gray-300
                           hover:bg-surface-hover transition-colors">取消</button>
              <button onClick={handleBalance} className="flex-1 btn-primary py-3">
                确认
              </button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  )
}
