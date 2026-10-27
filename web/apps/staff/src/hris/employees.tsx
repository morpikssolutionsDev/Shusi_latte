// Employee records (BR-HR-01, Management): employment details beyond the sign-in account, which
// stays on Admin → Crew (role, Stations, PIN, portrait).
import { Button, Card, Chip, Dialog, Field, Input, Select, Textarea } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { PencilIcon } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Avatar } from '../admin/crew';
import { PageHeader } from '../admin/layout';
import { useSave } from '../admin/api';
import { employeesQuery, hris, type Employee } from './api';

const TYPES = ['full_time', 'part_time', 'intern'] as const;

export function EmployeesPage() {
  const { t } = useTranslation();
  const list = useQuery(employeesQuery);
  const [edit, setEdit] = useState<Employee | null>(null);
  return (
    <>
      <PageHeader title={t('hris.employees.title')}>{t('hris.employees.hint')}</PageHeader>
      <Card className="overflow-x-auto p-0">
        <table className="w-full text-sm">
          <thead className="text-left text-muted-foreground">
            <tr className="border-b border-border-subtle">
              <th className="px-4 py-3 font-medium">{t('hris.schedule.crew')}</th>
              <th className="px-4 py-3 font-medium">{t('hris.employees.type')}</th>
              <th className="px-4 py-3 font-medium">{t('hris.employees.joined')}</th>
              <th className="px-4 py-3 font-medium">{t('hris.employees.phone')}</th>
              <th className="px-4 py-3 font-medium">{t('hris.employees.emergency')}</th>
              <th className="px-4 py-3" />
            </tr>
          </thead>
          <tbody>
            {list.data?.map((e) => (
              <tr key={e.id} className="h-14 border-b border-border-subtle" data-employee={e.crewId}>
                <td className="px-4">
                  <span className="flex items-center gap-3">
                    <Avatar member={e} size="size-9" />
                    <span>
                      <span className="font-semibold">{e.fullName}</span>{' '}
                      <span className="font-mono text-xs text-muted-foreground">{e.crewId}</span>
                      {!e.active && <Chip className="ml-2">{t('hris.employees.inactive')}</Chip>}
                      <span className="block text-xs text-muted-foreground">{t(`roles.${e.role}`)}</span>
                    </span>
                  </span>
                </td>
                <td className="px-4">{t(`hris.employees.types.${e.employmentType}`)}</td>
                <td className="tabular px-4">{e.joinedOn || '—'}</td>
                <td className="tabular px-4">{e.phone || '—'}</td>
                <td className="px-4">{e.emergencyContact || '—'}</td>
                <td className="px-4 text-right">
                  <Button size="sm" variant="ghost" aria-label={`${t('hris.employees.edit')} · ${e.fullName}`} onClick={() => setEdit(e)}><PencilIcon aria-hidden /></Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
      {edit && <EmployeeDialog employee={edit} onClose={() => setEdit(null)} />}
    </>
  );
}

function EmployeeDialog({ employee: e, onClose }: { employee: Employee; onClose: () => void }) {
  const { t } = useTranslation();
  const [v, setV] = useState({ phone: e.phone, employmentType: e.employmentType, joinedOn: e.joinedOn ?? '', emergencyContact: e.emergencyContact, notes: e.notes });
  const save = useSave(() => hris.saveEmployee(e.id, v), [['hris']], t('hris.employees.saved'));
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={`${t('hris.employees.edit')} · ${e.fullName}`}
      footer={<><Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button><Button disabled={save.isPending} onClick={() => save.mutate(undefined, { onSuccess: onClose })}>{t('admin.common.save')}</Button></>}>
      <div className="grid grid-cols-2 gap-3">
        <Field label={t('hris.employees.type')} htmlFor="emp-type">
          <Select id="emp-type" value={v.employmentType} onChange={(ev) => setV({ ...v, employmentType: ev.target.value as Employee['employmentType'] })}>
            {TYPES.map((x) => <option key={x} value={x}>{t(`hris.employees.types.${x}`)}</option>)}
          </Select>
        </Field>
        <Field label={t('hris.employees.joined')} htmlFor="emp-joined"><Input id="emp-joined" type="date" value={v.joinedOn} onChange={(ev) => setV({ ...v, joinedOn: ev.target.value })} /></Field>
        <Field label={t('hris.employees.phone')} htmlFor="emp-phone"><Input id="emp-phone" inputMode="tel" value={v.phone} onChange={(ev) => setV({ ...v, phone: ev.target.value })} /></Field>
        <Field label={t('hris.employees.emergency')} htmlFor="emp-emergency"><Input id="emp-emergency" value={v.emergencyContact} onChange={(ev) => setV({ ...v, emergencyContact: ev.target.value })} /></Field>
        <Field label={t('hris.employees.notes')} htmlFor="emp-notes" className="col-span-2"><Textarea id="emp-notes" rows={3} value={v.notes} onChange={(ev) => setV({ ...v, notes: ev.target.value })} /></Field>
      </div>
    </Dialog>
  );
}
