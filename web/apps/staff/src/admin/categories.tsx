import { api, unwrap } from '@sushi-latte/api-client';
import { Button, Card, Dialog, Field, Input, Select, Switch } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { PencilIcon, PlusIcon } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { catalogQuery, useSave, type Category, type DietaryTag } from './api';
import { PageHeader } from './layout';

/** Categories, Prep Area capacity and Dietary Tags (Management; BR-PC-06, BR-PC-07, TR-KT-02). */
export function CategoriesPage() {
  const { t } = useTranslation();
  const { data } = useQuery(catalogQuery);
  const [editCat, setEditCat] = useState<Category | null>(null);
  const [editTag, setEditTag] = useState<DietaryTag | null>(null);
  const saveCat = useSave(
    async (c: Category) =>
      c.id ? unwrap(await api.PUT('/catalog/categories/{id}', { params: { path: { id: c.id } }, body: c })) : unwrap(await api.POST('/catalog/categories', { body: c })),
    [['catalog']],
    t('admin.common.saved'),
  );
  const saveTag = useSave(
    async (tag: DietaryTag) =>
      tag.id ? unwrap(await api.PUT('/catalog/dietary-tags/{id}', { params: { path: { id: tag.id } }, body: tag })) : unwrap(await api.POST('/catalog/dietary-tags', { body: tag })),
    [['catalog']],
    t('admin.common.saved'),
  );
  const saveCapacity = useSave(
    async ({ id, parallelCapacity }: { id: string; parallelCapacity: number }) =>
      unwrap(await api.PUT('/catalog/prep-areas/{id}/capacity', { params: { path: { id } }, body: { parallelCapacity } })),
    [['catalog']],
    t('admin.common.saved'),
  );
  if (!data) return null;
  const areaName = (id: string) => data.prepAreas.find((a) => a.id === id)?.nameId ?? '';

  return (
    <>
      <PageHeader
        title={t('admin.categories.title')}
        actions={
          <Button onClick={() => setEditCat({ nameId: '', nameEn: '', prepAreaId: data.prepAreas[0]?.id ?? '', active: true, sort: (data.categories.length + 1) * 10 })}>
            <PlusIcon aria-hidden />
            {t('admin.categories.add')}
          </Button>
        }
      />
      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
        <Card className="p-0">
          <table className="w-full text-sm">
            <thead className="text-left text-xs uppercase tracking-wide text-muted-foreground">
              <tr className="border-b border-border-subtle">
                <th className="px-4 py-3">{t('admin.common.nameId')}</th>
                <th className="px-4 py-3">{t('admin.common.nameEn')}</th>
                <th className="px-4 py-3">{t('admin.categories.prepArea')}</th>
                <th className="px-4 py-3">{t('admin.editor.status')}</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody>
              {data.categories.map((c) => (
                <tr key={c.id} className="h-12 border-b border-border-subtle last:border-0">
                  <td className="px-4 font-semibold">{c.nameId}</td>
                  <td className="px-4">{c.nameEn}</td>
                  <td className="px-4">{areaName(c.prepAreaId)}</td>
                  <td className="px-4">{c.active ? t('admin.common.active') : t('admin.common.inactive')}</td>
                  <td className="px-2 text-right">
                    <Button variant="ghost" size="icon" aria-label={`${t('admin.common.edit')} ${c.nameId}`} onClick={() => setEditCat(c)}>
                      <PencilIcon aria-hidden />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>

        <Card className="flex flex-col gap-4 p-5">
          <div>
            <h2 className="text-lg font-bold">{t('admin.categories.capacity')}</h2>
            <p className="text-sm text-muted-foreground">{t('admin.categories.capacityHint')}</p>
          </div>
          {data.prepAreas.map((a) => (
            <Field key={a.id} label={a.nameId} htmlFor={`cap-${a.id}`}>
              <Input
                id={`cap-${a.id}`}
                type="number"
                min={1}
                max={20}
                className="tabular"
                defaultValue={a.parallelCapacity}
                onBlur={(e) => {
                  const v = Number(e.target.value);
                  if (v !== a.parallelCapacity) saveCapacity.mutate({ id: a.id, parallelCapacity: v });
                }}
              />
            </Field>
          ))}
        </Card>
      </div>

      <section className="mt-10">
        <PageHeader
          title={t('admin.categories.tagsTitle')}
          actions={
            <Button variant="secondary" onClick={() => setEditTag({ code: '', nameId: '', nameEn: '', active: true, sort: data.dietaryTags.length })}>
              <PlusIcon aria-hidden />
              {t('admin.categories.addTag')}
            </Button>
          }
        >
          {t('admin.editor.dietaryHint')}
        </PageHeader>
        <div className="flex flex-wrap gap-2">
          {data.dietaryTags.map((tag) => (
            <button
              key={tag.id}
              type="button"
              onClick={() => setEditTag(tag)}
              className="h-9 rounded-chip border px-3 text-sm font-semibold"
              style={{ background: 'var(--sl-color-dietary-warning-bg)', color: 'var(--sl-color-dietary-warning-fg)', borderColor: 'var(--sl-color-dietary-warning-border)', opacity: tag.active ? 1 : 0.5 }}
            >
              {tag.nameId} <span className="font-normal">· {tag.nameEn}</span>
            </button>
          ))}
        </div>
      </section>

      {editCat && (
        <Dialog
          open
          onOpenChange={(o) => !o && setEditCat(null)}
          title={editCat.id ? t('admin.common.edit') : t('admin.categories.add')}
          footer={
            <>
              <Button variant="secondary" onClick={() => setEditCat(null)}>{t('admin.common.cancel')}</Button>
              <Button disabled={saveCat.isPending} onClick={() => saveCat.mutate(editCat, { onSuccess: () => setEditCat(null) })}>{t('admin.common.save')}</Button>
            </>
          }
        >
          <div className="flex flex-col gap-4">
            <Field label={t('admin.common.nameId')} htmlFor="cat-id"><Input id="cat-id" value={editCat.nameId} onChange={(e) => setEditCat({ ...editCat, nameId: e.target.value })} /></Field>
            <Field label={t('admin.common.nameEn')} htmlFor="cat-en"><Input id="cat-en" value={editCat.nameEn} onChange={(e) => setEditCat({ ...editCat, nameEn: e.target.value })} /></Field>
            <Field label={t('admin.categories.prepArea')} htmlFor="cat-area">
              <Select id="cat-area" value={editCat.prepAreaId} onChange={(e) => setEditCat({ ...editCat, prepAreaId: e.target.value })}>
                {data.prepAreas.map((a) => <option key={a.id} value={a.id}>{a.nameId}</option>)}
              </Select>
            </Field>
            {editCat.id && (
              <label className="flex items-center gap-3 text-sm font-semibold">
                <Switch checked={editCat.active} onCheckedChange={(active) => setEditCat({ ...editCat, active })} />
                {t('admin.common.active')}
              </label>
            )}
          </div>
        </Dialog>
      )}

      {editTag && (
        <Dialog
          open
          onOpenChange={(o) => !o && setEditTag(null)}
          title={editTag.id ? t('admin.common.edit') : t('admin.categories.addTag')}
          footer={
            <>
              <Button variant="secondary" onClick={() => setEditTag(null)}>{t('admin.common.cancel')}</Button>
              <Button disabled={saveTag.isPending} onClick={() => saveTag.mutate(editTag, { onSuccess: () => setEditTag(null) })}>{t('admin.common.save')}</Button>
            </>
          }
        >
          <div className="flex flex-col gap-4">
            <Field label={t('admin.categories.code')} htmlFor="tag-code">
              <Input id="tag-code" className="font-mono" disabled={!!editTag.id} value={editTag.code} onChange={(e) => setEditTag({ ...editTag, code: e.target.value })} />
            </Field>
            <Field label={t('admin.common.nameId')} htmlFor="tag-id"><Input id="tag-id" value={editTag.nameId} onChange={(e) => setEditTag({ ...editTag, nameId: e.target.value })} /></Field>
            <Field label={t('admin.common.nameEn')} htmlFor="tag-en"><Input id="tag-en" value={editTag.nameEn} onChange={(e) => setEditTag({ ...editTag, nameEn: e.target.value })} /></Field>
            {editTag.id && (
              <label className="flex items-center gap-3 text-sm font-semibold">
                <Switch checked={editTag.active} onCheckedChange={(active) => setEditTag({ ...editTag, active })} />
                {t('admin.common.active')}
              </label>
            )}
          </div>
        </Dialog>
      )}
    </>
  );
}
